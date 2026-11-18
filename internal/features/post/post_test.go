package post

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

// ---------- Helpers ----------

func TestSlugify(t *testing.T) {
	assert.Equal(t, "halo-dunia-2026", Slugify("  Halo, Dunia!! 2026 "))
	assert.Equal(t, "artikel", Slugify("!!!"))
	assert.Equal(t, "caf", Slugify("Café"))
	long := Slugify(strings.Repeat("ab ", 150))
	assert.LessOrEqual(t, len(long), maxSlugLength)
	assert.False(t, strings.HasSuffix(long, "-"))
}

func TestReadingTime(t *testing.T) {
	assert.Equal(t, 1, ReadingTime(""))
	assert.Equal(t, 1, ReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, ReadingTime(strings.Repeat("kata ", 201)))
}

func TestNormalizeCategory(t *testing.T) {
	c, err := NormalizeCategory("")
	require.NoError(t, err)
	assert.Equal(t, "Umum", c)
	c, _ = NormalizeCategory("gaya hidup")
	assert.Equal(t, "Gaya Hidup", c)
	_, err = NormalizeCategory("Olahraga")
	assert.ErrorIs(t, err, ErrInvalidCategory)
}

func TestToPostResponseAuthor(t *testing.T) {
	r := ToPostResponse(&database.Post{ID: 1, User: database.User{ID: 2, Name: "A", Username: "a"}})
	require.NotNil(t, r.Author)
	assert.Equal(t, "a", r.Author.Username)
	assert.Nil(t, ToPostResponse(&database.Post{ID: 1}).Author)
}

// ---------- Repository ----------

func TestRepository(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)
	u1 := database.User{Name: "Budi", Username: "budi", Email: "b@x.com", Password: "x", Role: "member"}
	u2 := database.User{Name: "Sari", Username: "sari", Email: "s@x.com", Password: "x", Role: "member"}
	require.NoError(t, db.Create(&u1).Error)
	require.NoError(t, db.Create(&u2).Error)

	t1 := time.Now()
	p1 := &database.Post{UserID: u1.ID, Title: "Belajar Golang", Slug: "belajar-golang", Category: "Teknologi", Content: "golang gin", IsPublished: true, PublishedAt: &t1}
	p2 := &database.Post{UserID: u1.ID, Title: "Draf Bisnis", Slug: "draf-bisnis", Category: "Bisnis", Content: "rahasia"}
	p3 := &database.Post{UserID: u2.ID, Title: "Opini Pendidikan", Slug: "opini-pendidikan", Category: "Pendidikan", Content: "sekolah", IsPublished: true, PublishedAt: &t1}
	for _, p := range []*database.Post{p1, p2, p3} {
		require.NoError(t, repo.Create(p))
	}

	pub, err := repo.ListPublished(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, pub, 2)
	pub, _ = repo.ListPublished(ListFilter{Q: "GOLANG", Category: "Semua"})
	assert.Len(t, pub, 1)
	assert.Equal(t, "budi", pub[0].User.Username)
	pub, _ = repo.ListPublished(ListFilter{Category: "Pendidikan"})
	assert.Len(t, pub, 1)

	got, err := repo.FindPublishedBySlug("belajar-golang")
	require.NoError(t, err)
	assert.Equal(t, p1.ID, got.ID)
	_, err = repo.FindPublishedBySlug("draf-bisnis")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	mine, err := repo.ListByUser(u1.ID, ListFilter{})
	require.NoError(t, err)
	assert.Len(t, mine, 2)

	got, err = repo.FindByIDForUser(p2.ID, u1.ID)
	require.NoError(t, err)
	assert.Equal(t, "draf-bisnis", got.Slug)
	_, err = repo.FindByIDForUser(p2.ID, u2.ID)
	assert.Error(t, err)

	exists, err := repo.SlugExists("draf-bisnis", 0)
	require.NoError(t, err)
	assert.True(t, exists)
	exists, _ = repo.SlugExists("draf-bisnis", p2.ID)
	assert.False(t, exists)

	got.Title = "Draf Bisnis Baru"
	require.NoError(t, repo.Update(got))

	total, published, err := repo.CountByUser(u1.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, int64(1), published)

	all, err := repo.ListAll(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	users, err := repo.ListUsersWithStats()
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, int64(2), users[0].TotalPosts)
	assert.Equal(t, int64(1), users[0].PublishedPosts)

	ok, err := repo.UserExists(u1.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, _ = repo.UserExists(999)
	assert.False(t, ok)

	require.NoError(t, repo.Delete(p2))
	total, _, _ = repo.CountByUser(u1.ID)
	assert.Equal(t, int64(1), total)

	sqlDB, _ := db.DB()
	sqlDB.Close()
	_, _, err = repo.CountByUser(u1.ID)
	assert.Error(t, err)
}

// ---------- Service (fake repository) ----------

type fakeRepo struct {
	listPublished func(ListFilter) ([]database.Post, error)
	findSlug      func(string) (*database.Post, error)
	listByUser    func(uint, ListFilter) ([]database.Post, error)
	findForUser   func(uint, uint) (*database.Post, error)
	create        func(*database.Post) error
	update        func(*database.Post) error
	del           func(*database.Post) error
	slugExists    func(string, uint) (bool, error)
	count         func(uint) (int64, int64, error)
	listAll       func(ListFilter) ([]database.Post, error)
	users         func() ([]UserSummary, error)
	userExists    func(uint) (bool, error)
}

func (f *fakeRepo) ListPublished(x ListFilter) ([]database.Post, error)  { return f.listPublished(x) }
func (f *fakeRepo) FindPublishedBySlug(s string) (*database.Post, error) { return f.findSlug(s) }
func (f *fakeRepo) ListByUser(u uint, x ListFilter) ([]database.Post, error) {
	return f.listByUser(u, x)
}
func (f *fakeRepo) FindByIDForUser(id, u uint) (*database.Post, error) { return f.findForUser(id, u) }
func (f *fakeRepo) Create(p *database.Post) error                      { return f.create(p) }
func (f *fakeRepo) Update(p *database.Post) error                      { return f.update(p) }
func (f *fakeRepo) Delete(p *database.Post) error                      { return f.del(p) }
func (f *fakeRepo) SlugExists(s string, id uint) (bool, error)         { return f.slugExists(s, id) }
func (f *fakeRepo) CountByUser(u uint) (int64, int64, error)           { return f.count(u) }
func (f *fakeRepo) ListAll(x ListFilter) ([]database.Post, error)      { return f.listAll(x) }
func (f *fakeRepo) ListUsersWithStats() ([]UserSummary, error)         { return f.users() }
func (f *fakeRepo) UserExists(id uint) (bool, error)                   { return f.userExists(id) }

func samplePost(published bool) *database.Post {
	return &database.Post{ID: 5, UserID: 1, Title: "Judul Lama", Slug: "judul-lama", Category: "Umum", Content: "isi", IsPublished: published}
}

func okRepo() *fakeRepo {
	list := func() ([]database.Post, error) { return []database.Post{*samplePost(true)}, nil }
	return &fakeRepo{
		listPublished: func(ListFilter) ([]database.Post, error) { return list() },
		findSlug:      func(string) (*database.Post, error) { return samplePost(true), nil },
		listByUser:    func(uint, ListFilter) ([]database.Post, error) { return list() },
		findForUser:   func(uint, uint) (*database.Post, error) { return samplePost(false), nil },
		create:        func(p *database.Post) error { p.ID = 9; return nil },
		update:        func(*database.Post) error { return nil },
		del:           func(*database.Post) error { return nil },
		slugExists:    func(string, uint) (bool, error) { return false, nil },
		count:         func(uint) (int64, int64, error) { return 5, 3, nil },
		listAll:       func(ListFilter) ([]database.Post, error) { return list() },
		users:         func() ([]UserSummary, error) { return []UserSummary{{ID: 1}}, nil },
		userExists:    func(uint) (bool, error) { return true, nil },
	}
}

var req = PostRequest{Title: "Halo Dunia", Category: "teknologi", Content: "isi artikel", IsPublished: true}

func TestServiceReadOnly(t *testing.T) {
	r := okRepo()
	s := NewService(r)
	assert.Len(t, s.Categories(), 6)

	list, err := s.ListPublic(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	_, err = s.GetPublicBySlug("x")
	require.NoError(t, err)
	_, err = s.ListMine(1, ListFilter{})
	require.NoError(t, err)
	_, err = s.GetMine(1, 5)
	require.NoError(t, err)
	stats, err := s.Stats(1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.Draft)
	users, err := s.AdminUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)
	_, err = s.AdminPosts(ListFilter{})
	require.NoError(t, err)
	_, err = s.AdminUserPosts(1)
	require.NoError(t, err)

	r.listPublished = func(ListFilter) ([]database.Post, error) { return nil, errBoom }
	r.findSlug = func(string) (*database.Post, error) { return nil, gorm.ErrRecordNotFound }
	r.listByUser = func(uint, ListFilter) ([]database.Post, error) { return nil, errBoom }
	r.findForUser = func(uint, uint) (*database.Post, error) { return nil, errBoom }
	r.count = func(uint) (int64, int64, error) { return 0, 0, errBoom }
	r.listAll = func(ListFilter) ([]database.Post, error) { return nil, errBoom }
	_, err = s.ListPublic(ListFilter{})
	assert.ErrorIs(t, err, errBoom)
	_, err = s.GetPublicBySlug("x")
	assert.ErrorIs(t, err, ErrPostNotFound)
	_, err = s.ListMine(1, ListFilter{})
	assert.ErrorIs(t, err, errBoom)
	_, err = s.GetMine(1, 5)
	assert.ErrorIs(t, err, errBoom)
	_, err = s.Stats(1)
	assert.ErrorIs(t, err, errBoom)
	_, err = s.AdminPosts(ListFilter{})
	assert.ErrorIs(t, err, errBoom)

	r.userExists = func(uint) (bool, error) { return false, errBoom }
	_, err = s.AdminUserPosts(1)
	assert.ErrorIs(t, err, errBoom)
	r.userExists = func(uint) (bool, error) { return false, nil }
	_, err = s.AdminUserPosts(1)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestServiceCreate(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now = func() time.Time { return fixed }
	defer func() { now = time.Now }()

	r := okRepo()
	taken := map[string]bool{"halo-dunia": true, "halo-dunia-2": true}
	r.slugExists = func(s string, _ uint) (bool, error) { return taken[s], nil }
	res, err := NewService(r).Create(1, req)
	require.NoError(t, err)
	assert.Equal(t, "halo-dunia-3", res.Slug)
	assert.Equal(t, "Teknologi", res.Category)
	assert.True(t, res.IsPublished)
	assert.Equal(t, fixed, *res.PublishedAt)

	bad := req
	bad.Category = "Olahraga"
	_, err = NewService(r).Create(1, bad)
	assert.ErrorIs(t, err, ErrInvalidCategory)

	r.slugExists = func(string, uint) (bool, error) { return false, errBoom }
	_, err = NewService(r).Create(1, req)
	assert.ErrorIs(t, err, errBoom)

	r = okRepo()
	r.create = func(*database.Post) error { return errBoom }
	_, err = NewService(r).Create(1, req)
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceUpdate(t *testing.T) {
	r := okRepo()
	res, err := NewService(r).Update(1, 5, req)
	require.NoError(t, err)
	assert.Equal(t, "halo-dunia", res.Slug)
	assert.True(t, res.IsPublished)
	assert.NotNil(t, res.PublishedAt)

	same := req
	same.Title = "Judul Lama"
	same.IsPublished = false
	res, err = NewService(r).Update(1, 5, same)
	require.NoError(t, err)
	assert.Equal(t, "judul-lama", res.Slug)
	assert.Nil(t, res.PublishedAt)

	r.findForUser = func(uint, uint) (*database.Post, error) { return nil, gorm.ErrRecordNotFound }
	_, err = NewService(r).Update(1, 5, req)
	assert.ErrorIs(t, err, ErrPostNotFound)

	r = okRepo()
	bad := req
	bad.Category = "X"
	_, err = NewService(r).Update(1, 5, bad)
	assert.ErrorIs(t, err, ErrInvalidCategory)

	r.slugExists = func(string, uint) (bool, error) { return false, errBoom }
	_, err = NewService(r).Update(1, 5, req)
	assert.ErrorIs(t, err, errBoom)

	r = okRepo()
	r.update = func(*database.Post) error { return errBoom }
	_, err = NewService(r).Update(1, 5, req)
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceToggleDelete(t *testing.T) {
	r := okRepo()
	res, err := NewService(r).TogglePublish(1, 5)
	require.NoError(t, err)
	assert.True(t, res.IsPublished)

	r.findForUser = func(uint, uint) (*database.Post, error) { return samplePost(true), nil }
	res, _ = NewService(r).TogglePublish(1, 5)
	assert.False(t, res.IsPublished)
	assert.Nil(t, res.PublishedAt)

	r.update = func(*database.Post) error { return errBoom }
	_, err = NewService(r).TogglePublish(1, 5)
	assert.ErrorIs(t, err, errBoom)

	r.findForUser = func(uint, uint) (*database.Post, error) { return nil, gorm.ErrRecordNotFound }
	_, err = NewService(r).TogglePublish(1, 5)
	assert.ErrorIs(t, err, ErrPostNotFound)
	assert.ErrorIs(t, NewService(r).Delete(1, 5), ErrPostNotFound)

	r = okRepo()
	assert.NoError(t, NewService(r).Delete(1, 5))
}

// ---------- Handler (fake service) ----------

type fakeSvc struct {
	err       error
	published bool
}

func (f *fakeSvc) one() (*PostResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &PostResponse{ID: 1, IsPublished: f.published}, nil
}
func (f *fakeSvc) many() ([]PostResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []PostResponse{{ID: 1}}, nil
}
func (f *fakeSvc) Categories() []string                              { return database.Categories }
func (f *fakeSvc) ListPublic(ListFilter) ([]PostResponse, error)     { return f.many() }
func (f *fakeSvc) GetPublicBySlug(string) (*PostResponse, error)     { return f.one() }
func (f *fakeSvc) ListMine(uint, ListFilter) ([]PostResponse, error) { return f.many() }
func (f *fakeSvc) GetMine(uint, uint) (*PostResponse, error)         { return f.one() }
func (f *fakeSvc) Create(uint, PostRequest) (*PostResponse, error)   { return f.one() }
func (f *fakeSvc) Update(uint, uint, PostRequest) (*PostResponse, error) {
	return f.one()
}
func (f *fakeSvc) TogglePublish(uint, uint) (*PostResponse, error) { return f.one() }
func (f *fakeSvc) Delete(uint, uint) error                         { return f.err }
func (f *fakeSvc) Stats(uint) (*StatsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &StatsResponse{}, nil
}
func (f *fakeSvc) AdminUsers() ([]UserSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []UserSummary{}, nil
}
func (f *fakeSvc) AdminPosts(ListFilter) ([]PostResponse, error) { return f.many() }
func (f *fakeSvc) AdminUserPosts(uint) ([]PostResponse, error)   { return f.many() }

func engine(svc Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/categories", h.Categories)
	r.GET("/public", h.ListPublic)
	r.GET("/public/:slug", h.GetPublic)
	a := r.Group("/posts", middleware.Auth("s"))
	a.GET("", h.ListMine)
	a.POST("", h.Create)
	a.GET("/stats", h.Stats)
	a.GET("/:id", h.GetMine)
	a.PUT("/:id", h.Update)
	a.PATCH("/:id/toggle-publish", h.TogglePublish)
	a.DELETE("/:id", h.Delete)
	r.GET("/admin/users", h.AdminUsers)
	r.GET("/admin/posts", h.AdminPosts)
	r.GET("/admin/users/:id/posts", h.AdminUserPosts)
	return r
}

var token = middleware.GenerateToken("s", 1, "budi", "member", time.Hour)

func call(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	rq := httptest.NewRequest(method, path, &buf)
	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, rq)
	return w
}

type tc struct {
	method, path string
	body         any
}

var allRoutes = []tc{
	{"GET", "/public", nil},
	{"GET", "/public/abc", nil},
	{"GET", "/posts", nil},
	{"POST", "/posts", req},
	{"GET", "/posts/stats", nil},
	{"GET", "/posts/1", nil},
	{"PUT", "/posts/1", req},
	{"PATCH", "/posts/1/toggle-publish", nil},
	{"DELETE", "/posts/1", nil},
	{"GET", "/admin/users", nil},
	{"GET", "/admin/posts?q=x&category=Bisnis", nil},
	{"GET", "/admin/users/1/posts", nil},
}

func TestHandlerSuccess(t *testing.T) {
	r := engine(&fakeSvc{})
	assert.Equal(t, http.StatusOK, call(r, "GET", "/categories", nil).Code)
	for _, c := range allRoutes {
		w := call(r, c.method, c.path, c.body)
		assert.Less(t, w.Code, 300, c.method+" "+c.path)
	}
	w := call(engine(&fakeSvc{published: true}), "PATCH", "/posts/1/toggle-publish", nil)
	assert.Contains(t, w.Body.String(), "diterbitkan")
}

func TestHandlerErrors(t *testing.T) {
	for _, e := range []error{ErrPostNotFound, errBoom} {
		r := engine(&fakeSvc{err: e})
		for _, c := range allRoutes {
			w := call(r, c.method, c.path, c.body)
			assert.GreaterOrEqual(t, w.Code, 400, c.method+" "+c.path)
		}
	}
	r := engine(&fakeSvc{err: ErrInvalidCategory})
	assert.Equal(t, http.StatusBadRequest, call(r, "POST", "/posts", req).Code)
}

func TestHandlerBadInput(t *testing.T) {
	r := engine(&fakeSvc{})
	for _, path := range []string{"/posts/abc", "/posts/0"} {
		assert.Equal(t, http.StatusBadRequest, call(r, "GET", path, nil).Code)
		assert.Equal(t, http.StatusBadRequest, call(r, "PUT", path, req).Code)
		assert.Equal(t, http.StatusBadRequest, call(r, "DELETE", path, nil).Code)
		assert.Equal(t, http.StatusBadRequest, call(r, "PATCH", path+"/toggle-publish", nil).Code)
	}
	assert.Equal(t, http.StatusBadRequest, call(r, "GET", "/admin/users/x/posts", nil).Code)
	assert.Equal(t, http.StatusBadRequest, call(r, "POST", "/posts", "{}").Code)
	assert.Equal(t, http.StatusBadRequest, call(r, "PUT", "/posts/1", "{}").Code)
}
