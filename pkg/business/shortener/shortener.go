// Package shortener implements saving/extracting URL to/from database and
// encoding/decoding URL database id to BASE62 formatted string.
package shortener

import (
	"context"
	"errors"
	"fmt"

	"github.com/jxskiss/base62"

	"github.com/jmoiron/sqlx"
)

// EncShift is added to URL's id before encoding it to BASE62.
const EncShift = 1024 * 1024

var (
	DecodeErr   = errors.New("code is incorrect")
	EncShiftErr = errors.New("code is less than encoding shift")
)

// Engine contains the database for storing URLs.
type Engine struct {
	DB *sqlx.DB
}

// New constructs a new Engine.
func New(db *sqlx.DB) Engine {
	return Engine{DB: db}
}

// Shorten saves a URL to the database and returns its id encoded to BASE62 string.
func (e Engine) Shorten(ctx context.Context, url string) (string, error) {
	const sql = `INSERT INTO urls(url, date_created) VALUES ($1, NOW()) 
                	ON CONFLICT(url) DO UPDATE SET date_created = NOW() RETURNING id`

	var id int64
	err := e.DB.QueryRowxContext(ctx, sql, url).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("query %s: %w", sql, err)
	}

	return Encode(id), nil
}

// Expand takes the BASE62 code, decodes it and finds a corresponding URL in the database.
func (e Engine) Expand(ctx context.Context, code string) (string, error) {
	const sql = `SELECT url FROM urls WHERE id = $1`

	id, err := Decode(code)
	if err != nil {
		return "", DecodeErr
	}

	var url string
	err = e.DB.QueryRowxContext(ctx, sql, id).Scan(&url)
	if err != nil {
		return "", fmt.Errorf("query %s: %w", sql, err)
	}

	return url, nil
}

func Encode(id int64) string {
	return base62.EncodeToString(base62.FormatInt(id + EncShift))
}

func Decode(code string) (int64, error) {
	bb, err := base62.DecodeString(code)
	if err != nil {
		return 0, fmt.Errorf("base62.Decode(%s): %w", code, err)
	}

	id, err := base62.ParseInt(bb)
	if err != nil {
		return 0, fmt.Errorf("base62.ParseUint: %w", err)
	}

	if id <= EncShift {
		return 0, EncShiftErr
	}

	return id - EncShift, nil
}
