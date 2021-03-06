package nicolive

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

var (
	userInfoResponseOkName = "テスト名前"
	userInfoResponseOkThum = "testurl"
	userInfoResponseOk     = `<?xml version="1.0" encoding="UTF-8"?>
<nicovideo_user_response status="ok">
  <user>
    <id>1</id>
    <nickname>` + userInfoResponseOkName + `</nickname>
    <thumbnail_url>` + userInfoResponseOkThum + `</thumbnail_url>
  </user>
  <vita_option>
    <user_secret>0</user_secret>
  </vita_option>
  <additionals/>
</nicovideo_user_response>`
	userInfoResponseNotFound = `<?xml version="1.0" encoding="UTF-8"?>
<nicovideo_user_response status="fail"><error><code>NOT_FOUND</code><description>ユーザーが見つかりません</description></error></nicovideo_user_response>`
)

func TestUserFetchInfo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, userInfoResponseOk)
		},
	)
	mux.HandleFunc("/notfound",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, userInfoResponseNotFound)
		},
	)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := &Account{Usersession: "usersession_example"}
	a.UpdateClient()

	u, nerr := fetchUserInfoImpl(ts.URL+"/ok", a)
	if nerr != nil {
		t.Fatal(nerr)
	}
	if u.Name != userInfoResponseOkName {
		t.Fatalf("Should be %v but %v", userInfoResponseOkName, u.Name)
	}
	if u.ThumbnailURL != userInfoResponseOkThum {
		t.Fatalf("Should be %v but %v", userInfoResponseOkThum, u.ThumbnailURL)
	}

	_, nerr = fetchUserInfoImpl(ts.URL+"/notfound", a)
	if nerr == nil {
		t.Fatal(nerr)
	}
}

func TestUserDB(t *testing.T) {
	dir, err := ioutil.TempDir("", "nagome")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	db, err := NewUserDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = db.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	su := &User{
		ID:           "testid",
		Name:         "name",
		CreateTime:   time.Now(),
		Is184:        false,
		ThumbnailURL: "url",
	}

	err = db.Store(su)
	if err != nil {
		t.Fatal(err)
	}

	fu, err := db.Fetch("testid")
	if err != nil {
		t.Fatal(err)
	}
	if !fu.Equal(su) {
		t.Fatalf("Should be %v but %v", su, fu)
	}

	fu, err = db.Fetch("fail")
	if err == nil {
		t.Fatalf("Should be failed")
	}
	nerr, ok := err.(Error)
	if !ok {
		t.Fatalf("Should be nicolive.Error")
	}
	if nerr.Type() != ErrDBUserNotFound {
		t.Fatalf("Should be %v but %v", ErrDBUserNotFound, nerr.Type())
	}

	err = db.Remove("testid")
	if err != nil {
		t.Fatal(err)
	}
	fu, err = db.Fetch("testid")
	if err == nil {
		t.Fatalf("Should be failed")
	}
	if fu != nil {
		t.Fatalf("Should be %v but %v", nil, fu)
	}
}
