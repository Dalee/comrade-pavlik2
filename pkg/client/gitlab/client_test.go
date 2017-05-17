package gitlab

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

var (
	testClientTokenValid   = "token"
	testClientTokenInvalid = "invalid-token"
)

func TestNewClient_InvalidEndpoint(t *testing.T) {
	ts := createTestHttpServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`Not found`))
	})
	defer ts.Close()

	// run test
	client, err := NewClient(ts.URL, testClientTokenValid)

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_V3_InvalidToken(t *testing.T) {
	ts := createTestGitLabAPIV3(t, nil)
	defer ts.Close()

	// run test
	client, err := NewClient(ts.URL, testClientTokenInvalid)

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_V3(t *testing.T) {
	ts := createTestGitLabAPIV3(t, nil)
	defer ts.Close()

	// run test
	client, err := NewClient(ts.URL, testClientTokenValid)

	assert.Nil(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, false, client.HasV4Support)
	assert.Equal(t, true, client.HasV3Support)
	assert.Equal(t, "/api/v3", client.APIPrefix)
}

func TestNewClient_V4_InvalidToken(t *testing.T) {
	ts := createTestGitLabAPIV4(t, nil)
	defer ts.Close()

	// run test
	client, err := NewClient(ts.URL, testClientTokenInvalid)

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_V4(t *testing.T) {
	ts := createTestGitLabAPIV4(t, nil)
	defer ts.Close()

	// run test
	client, err := NewClient(ts.URL, testClientTokenValid)

	assert.Nil(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, true, client.HasV4Support)
	assert.Equal(t, false, client.HasV3Support)
	assert.Equal(t, "/api/v4", client.APIPrefix)
}

func createTestGitLabAPIV3(t *testing.T, fn http.HandlerFunc) *httptest.Server {
	ts := createTestHttpServer(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("PRIVATE-TOKEN")

		if r.URL.RawQuery != "" {
			t.Logf("%v, %v?%v", r.Method, r.URL.Path, r.URL.RawQuery)
		} else {
			t.Logf("%v, %v", r.Method, r.URL.Path)
		}

		//
		// Simulate GitLab 8.5 behaviour
		// API v4: not supported
		// API v3: supported
		//
		if strings.HasPrefix(r.URL.Path, "/api/v4/") {
			// GitLab with invalid token will offer redirect to sign_in
			if token != testClientTokenValid {
				w.Header().Set("Location", "/users/sign_in")
				w.WriteHeader(http.StatusFound)
				return
			}

			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`Not found`))
			return
		}

		if token != testClientTokenValid {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`Access denied`))
			return
		}

		if r.Method == "HEAD" && r.URL.Path == "/api/v3/user" {
			w.WriteHeader(http.StatusOK)
			return
		}

		//
		// Pass to testing custom handler (if provided)
		// in order to test different responses
		//
		if fn != nil {
			fn(w, r)
		}
	})

	return ts
}

func createTestGitLabAPIV4(t *testing.T, fn http.HandlerFunc) *httptest.Server {
	ts := createTestHttpServer(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("PRIVATE-TOKEN")

		t.Logf("Method: %v", r.Method)
		t.Logf("Path: %v", r.URL.Path)
		t.Logf("Is token valid?: %v", token == testClientTokenValid)

		//
		// Simulate GitLab > 9.3 behaviour
		// API v4: supported
		// API v3: not supported
		//
		if token != testClientTokenValid {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`Access denied`))
			return
		}

		if r.Method == "HEAD" && r.URL.Path == "/api/v4/user" {
			w.WriteHeader(http.StatusOK)
			return
		}

		//
		// Pass to testing custom handler (if provided)
		// in order to test different responses
		//
		if fn != nil {
			fn(w, r)
		}
	})

	return ts
}

func createTestHttpServer(fn http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(fn))
}

func getTestRawDataFromFile(t *testing.T, filename string) []byte {
	file, err := filepath.Abs(filename)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	return data
}
