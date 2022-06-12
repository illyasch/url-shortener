package handlers_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ardanlabs/conf/v3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/illyasch/url-shortener/cmd/url-shortener/handlers"
	"github.com/illyasch/url-shortener/pkg/business/shortener"
	"github.com/illyasch/url-shortener/pkg/data/database"
	"github.com/illyasch/url-shortener/pkg/sys/logger"
)

var (
	postgresDB *sqlx.DB
	stdLgr     *zap.SugaredLogger
)

func TestMain(m *testing.M) {
	var err error
	stdLgr, err = logger.New("shortener")
	if err != nil {
		log.Fatal(err)
	}

	cfg := struct {
		conf.Version
		DB struct {
			User         string `conf:"default:postgres"`
			Password     string `conf:"default:nimda,mask"`
			Host         string `conf:"default:localhost"`
			Name         string `conf:"default:postgres"`
			MaxIdleConns int    `conf:"default:0"`
			MaxOpenConns int    `conf:"default:0"`
			DisableTLS   bool   `conf:"default:true"`
		}
	}{
		Version: conf.Version{
			Build: "test",
			Desc:  "Copyright Ilya Scheblanov",
		},
	}

	const prefix = "SHORTENER"
	_, err = conf.Parse(prefix, &cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(cfg)

	postgresDB, err = database.Open(database.Config{
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		Host:         cfg.DB.Host,
		Name:         cfg.DB.Name,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		DisableTLS:   cfg.DB.DisableTLS,
	})
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(m.Run())
}

func TestAPIConfig_handleShorten(t *testing.T) {
	t.Parallel()
	cfg := handlers.APIConfig{
		Log: stdLgr,
		DB:  postgresDB,
	}

	t.Run("successful URL shortening", func(t *testing.T) {
		t.Parallel()

		expURL := "https://www.testurl.com/foo/bar/" + uuid.NewString()
		vals := url.Values{}
		vals.Set("url", expURL)
		r := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(vals.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		var got struct {
			Code string `json:"code"`
		}
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Code)

		var id int64
		err = cfg.DB.QueryRowx("SELECT id FROM urls WHERE url = $1", expURL).Scan(&id)
		require.NoError(t, err)
		assert.Equal(t, shortener.Encode(id), got.Code)
	})

	t.Run("URL validation error", func(t *testing.T) {
		t.Parallel()

		expURL := "hjef"
		vals := url.Values{}
		vals.Set("url", expURL)
		r := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(vals.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var got struct {
			Error string `json:"error"`
		}
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Error)
	})

	t.Run("previously existed URL", func(t *testing.T) {
		t.Parallel()

		var expID int64
		var expURL string
		err := cfg.DB.QueryRowx("SELECT id, url FROM urls LIMIT 1").Scan(&expID, &expURL)
		if err != nil {
			t.Skip("select any existed row", err)
		}

		vals := url.Values{}
		vals.Set("url", expURL)
		r := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(vals.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		var got struct {
			Code string `json:"code"`
		}
		err = json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Code)

		require.NoError(t, err)
		assert.Equal(t, shortener.Encode(expID), got.Code)
	})
}

func TestAPIConfig_handleExpand(t *testing.T) {
	t.Parallel()
	cfg := handlers.APIConfig{
		Log: stdLgr,
		DB:  postgresDB,
	}

	t.Run("URL code validation error", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "/ff", nil)
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var got struct {
			Error string `json:"error"`
		}
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Error)
	})

	t.Run("successful finding an URL with the code", func(t *testing.T) {
		t.Parallel()

		var expID int64
		var expURL string
		err := cfg.DB.QueryRowx("SELECT id, url FROM urls LIMIT 1").Scan(&expID, &expURL)
		if err != nil {
			t.Skip("select any existed row", err)
		}

		r := httptest.NewRequest(http.MethodGet, "/"+shortener.Encode(expID), nil)
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		var got struct {
			URL string `json:"url"`
		}
		err = json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.Equal(t, expURL, got.URL)
	})

	t.Run("incorrect input code", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString(), nil)
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var got struct {
			Error string `json:"error"`
		}
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Error)
	})

	t.Run("not found an URL error", func(t *testing.T) {
		t.Parallel()

		var id int64
		var err error
		// trying to find an id which does not exist in our DB
		for i := 0; i < 10; i++ {
			id = rand.Int63n(shortener.EncShift*10) + shortener.EncShift*10

			var url string
			err = cfg.DB.QueryRowx("SELECT url FROM urls WHERE id = $1", id).Scan(&url)
			if errors.Is(sql.ErrNoRows, err) {
				break
			}
		}
		if err == nil {
			t.Skip("can not generate an id which does not exist in our DB", err)
		}

		r := httptest.NewRequest(http.MethodGet, "/"+shortener.Encode(id), nil)
		w := httptest.NewRecorder()

		cfg.Router().ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var got struct {
			Error string `json:"error"`
		}
		err = json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Error)
	})
}
