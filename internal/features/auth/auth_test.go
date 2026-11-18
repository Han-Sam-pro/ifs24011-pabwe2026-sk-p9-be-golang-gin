package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/database"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var errBoom = errors.New("boom")

func newDB(t *testing.T) *gorm.DB {
	db, err := database.ConnectSQLite("file::memory:")
	require.NoError(t, err)
	return db
}

// ---------- Repository ----------

func TestRepository(t *testing.T) {
	repo := NewRepository(newDB(t))
	u := &database.User{Name: "Budi", Username: "budi", Email: "budi@x.com", Password: "h", Role: "member"}
	require.NoError(t, repo.Create(u))

	got, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "budi", got.Username)

	_, err = repo.FindByID(999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	got, err = repo.FindByLogin("budi@x.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
	_, err = repo.FindByLogin("nobody")
	assert.Error(t, err)

	taken, err := repo.IsTaken("budi", "other@x.com", 0)
	require.NoError(t, err)
	assert.True(t, taken)
	taken, _ = repo.IsTaken("budi", "budi@x.com", u.ID)
	assert.False(t, taken)

	got.Name = "Budi S"
	require.NoError(t, repo.Update(got))
	got, _ = repo.FindByID(u.ID)
	assert.Equal(t, "Budi S", got.Name)
}

// ---------- Service (fake repository) ----------

type fakeRepo struct {
	create      func(u *database.User) error
	findByID    func(id uint) (*database.User, error)
	findByLogin func(identity string) (*database.User, error)
	isTaken     func(username, email string, excludeID uint) (bool, error)
	update      func(u *database.User) error
}

func (f *fakeRepo) Create(u *database.User) error                { return f.create(u) }
func (f *fakeRepo) FindByID(id uint) (*database.User, error)     { return f.findByID(id) }
func (f *fakeRepo) FindByLogin(i string) (*database.User, error) { return f.findByLogin(i) }
func (f *fakeRepo) IsTaken(u, e string, id uint) (bool, error)   { return f.isTaken(u, e, id) }
func (f *fakeRepo) Update(u *database.User) error                { return f.update(u) }

func okRepo() *fakeRepo {
	hash, _ := database.HashPassword("secret1")
	user := func() *database.User {
		return &database.User{ID: 1, Name: "Budi", Username: "budi", Email: "budi@x.com", Password: hash, Role: "member"}
	}
	return &fakeRepo{
		create:      func(u *database.User) error { u.ID = 1; return nil },
		findByID:    func(uint) (*database.User, error) { return user(), nil },
		findByLogin: func(string) (*database.User, error) { return user(), nil },
		isTaken:     func(string, string, uint) (bool, error) { return false, nil },
		update:      func(*database.User) error { return nil },
	}
}

var regReq = RegisterRequest{Name: " Budi ", Username: "Budi", Email: "BUDI@x.com", Password: "secret1"}

func TestServiceRegister(t *testing.T) {
	r := okRepo()
	res, err := NewService(r, "s").Register(regReq)
	require.NoError(t, err)
	assert.Equal(t, "budi", res.Username)
	assert.Equal(t, "budi@x.com", res.Email)
	assert.Equal(t, "Budi", res.Name)

	r.isTaken = func(string, string, uint) (bool, error) { return false, errBoom }
	_, err = NewService(r, "s").Register(regReq)
	assert.ErrorIs(t, err, errBoom)

	r.isTaken = func(string, string, uint) (bool, error) { return true, nil }
	_, err = NewService(r, "s").Register(regReq)
	assert.ErrorIs(t, err, ErrConflict)

	r = okRepo()
	long := regReq
	long.Password = strings.Repeat("x", 80)
	_, err = NewService(r, "s").Register(long)
	assert.Error(t, err)

	r.create = func(*database.User) error { return errBoom }
	_, err = NewService(r, "s").Register(regReq)
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceLogin(t *testing.T) {
	r := okRepo()
	res, err := NewService(r, "s").Login(LoginRequest{Username: "budi", Password: "secret1"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token)
	claims, err := middleware.ParseToken("s", res.Token)
	require.NoError(t, err)
	assert.Equal(t, uint(1), claims.UserID)

	_, err = NewService(r, "s").Login(LoginRequest{Username: "budi", Password: "wrong"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	r.findByLogin = func(string) (*database.User, error) { return nil, gorm.ErrRecordNotFound }
	_, err = NewService(r, "s").Login(LoginRequest{Username: "x", Password: "y"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	r.findByLogin = func(string) (*database.User, error) { return nil, errBoom }
	_, err = NewService(r, "s").Login(LoginRequest{Username: "x", Password: "y"})
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceProfile(t *testing.T) {
	r := okRepo()
	res, err := NewService(r, "s").Profile(1)
	require.NoError(t, err)
	assert.Equal(t, "budi", res.Username)

	r.findByID = func(uint) (*database.User, error) { return nil, gorm.ErrRecordNotFound }
	_, err = NewService(r, "s").Profile(1)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestServiceUpdateProfile(t *testing.T) {
	req := UpdateProfileRequest{Name: "Budi Baru", Email: "NEW@x.com", Password: "newpass"}
	r := okRepo()
	var saved *database.User
	r.update = func(u *database.User) error { saved = u; return nil }
	res, err := NewService(r, "s").UpdateProfile(1, req)
	require.NoError(t, err)
	assert.Equal(t, "new@x.com", res.Email)
	assert.True(t, database.CheckPassword(saved.Password, "newpass"))

	noPw := req
	noPw.Password = ""
	_, err = NewService(r, "s").UpdateProfile(1, noPw)
	require.NoError(t, err)
	assert.True(t, database.CheckPassword(saved.Password, "secret1"))

	r.findByID = func(uint) (*database.User, error) { return nil, gorm.ErrRecordNotFound }
	_, err = NewService(r, "s").UpdateProfile(1, req)
	assert.ErrorIs(t, err, ErrUserNotFound)

	r = okRepo()
	r.isTaken = func(string, string, uint) (bool, error) { return false, errBoom }
	_, err = NewService(r, "s").UpdateProfile(1, req)
	assert.ErrorIs(t, err, errBoom)

	r.isTaken = func(string, string, uint) (bool, error) { return true, nil }
	_, err = NewService(r, "s").UpdateProfile(1, req)
	assert.ErrorIs(t, err, ErrConflict)

	r = okRepo()
	long := req
	long.Password = strings.Repeat("x", 80)
	_, err = NewService(r, "s").UpdateProfile(1, long)
	assert.Error(t, err)

	r.update = func(*database.User) error { return errBoom }
	_, err = NewService(r, "s").UpdateProfile(1, req)
	assert.ErrorIs(t, err, errBoom)
}

// ---------- Handler (fake service) ----------

type fakeSvc struct{ err error }

func (f *fakeSvc) Register(RegisterRequest) (*UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &UserResponse{ID: 1, Username: "budi"}, nil
}
func (f *fakeSvc) Login(LoginRequest) (*LoginResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &LoginResponse{Token: "tok"}, nil
}
func (f *fakeSvc) Profile(uint) (*UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &UserResponse{ID: 1}, nil
}
func (f *fakeSvc) UpdateProfile(uint, UpdateProfileRequest) (*UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &UserResponse{ID: 1}, nil
}

func engine(svc Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	authed := r.Group("/", middleware.Auth("s"))
	authed.POST("/logout", h.Logout)
	authed.GET("/profile", h.GetProfile)
	authed.PUT("/profile", h.UpdateProfile)
	return r
}

func call(r http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandlerRegisterLogin(t *testing.T) {
	r := engine(&fakeSvc{})
	assert.Equal(t, http.StatusCreated, call(r, "POST", "/register", regReq, "").Code)
	assert.Equal(t, http.StatusBadRequest, call(r, "POST", "/register", "{bad", "").Code)
	assert.Equal(t, http.StatusOK, call(r, "POST", "/login", LoginRequest{"a", "b"}, "").Code)
	assert.Equal(t, http.StatusBadRequest, call(r, "POST", "/login", "{}", "").Code)

	cases := map[error]int{
		ErrConflict:           http.StatusConflict,
		ErrInvalidCredentials: http.StatusUnauthorized,
		ErrUserNotFound:       http.StatusNotFound,
		errBoom:               http.StatusInternalServerError,
	}
	for e, code := range cases {
		r := engine(&fakeSvc{err: e})
		assert.Equal(t, code, call(r, "POST", "/register", regReq, "").Code)
	}
	assert.Equal(t, http.StatusUnauthorized, call(engine(&fakeSvc{err: ErrInvalidCredentials}), "POST", "/login", LoginRequest{"a", "b"}, "").Code)
}

func TestHandlerProfileAndLogout(t *testing.T) {
	tok := middleware.GenerateToken("s", 1, "budi", "member", time.Hour)
	r := engine(&fakeSvc{})
	assert.Equal(t, http.StatusOK, call(r, "GET", "/profile", nil, tok).Code)
	upd := UpdateProfileRequest{Name: "Budi", Email: "b@x.com"}
	assert.Equal(t, http.StatusOK, call(r, "PUT", "/profile", upd, tok).Code)
	assert.Equal(t, http.StatusBadRequest, call(r, "PUT", "/profile", "{}", tok).Code)

	bad := engine(&fakeSvc{err: ErrUserNotFound})
	assert.Equal(t, http.StatusNotFound, call(bad, "GET", "/profile", nil, tok).Code)
	assert.Equal(t, http.StatusNotFound, call(bad, "PUT", "/profile", upd, tok).Code)

	assert.Equal(t, http.StatusOK, call(r, "POST", "/logout", nil, tok).Code)
	assert.Equal(t, http.StatusUnauthorized, call(r, "GET", "/profile", nil, tok).Code)
}
