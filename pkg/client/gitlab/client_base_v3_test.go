package gitlab

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestGitLabClient_V3_ProjectList(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/project/list_v3.json"))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	projectList, err := client.GetProjectList()
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, projectList, 2)

	// check first
	assert.Equal(t, "Diaspora Client", projectList[0].Name)
	assert.Equal(t, "Puppet", projectList[1].Name)
}

func TestGitLabClient_V3_ProjectListPagination(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects" {
			page := r.URL.Query().Get("page")
			if page == "" {
				// first query
				w.Header().Set("X-Next-Page", "2")
				w.Header().Set("X-Total-Pages", "2")
				w.WriteHeader(http.StatusOK)
				w.Write(getTestRawDataFromFile(t, "./test-data/project/list_v3.json"))
			} else {
				// second query, no more pages
				w.Header().Set("X-Next-Page", "")
				w.WriteHeader(http.StatusOK)
				w.Write(getTestRawDataFromFile(t, "./test-data/project/list_v3.json"))
			}
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	projectList, err := client.GetProjectList()
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, projectList, 4)

	// check first
	assert.Equal(t, "Diaspora Client", projectList[0].Name)
	assert.Equal(t, "Puppet", projectList[1].Name)
}

func TestGitLabClient_V3_GetProjectById(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects/4" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/project/item_v3.json"))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	project, err := client.GetProjectById(4)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, project.ID)
	assert.Equal(t, "Diaspora Client", project.Name)
}

func TestGitLabClient_V3_GetProjectById_Error(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects/4" {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(``))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	project, err := client.GetProjectById(4)

	assert.Error(t, err)
	assert.Nil(t, project)
}

func TestGitLabClient_V3_ProjectList_Error(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects" {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(``))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	projectList, err := client.GetProjectList()

	assert.Error(t, err)
	assert.Nil(t, projectList)
}

func TestGitLabClient_V3_ProjectList_Redirect(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects" {
			w.Header().Set("Location", "/users/sign_in")
			w.WriteHeader(http.StatusFound)
			w.Write([]byte(``))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	projectList, err := client.GetProjectList()

	assert.Error(t, err)
	assert.Nil(t, projectList)
}

func TestGitLabClient_V3_GetTagList(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects/4/repository/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/tag/list_v3.json"))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	project := &Project{
		ID: 4,
	}

	tagList, err := client.GetTagList(project)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, tagList, 1)
	assert.Equal(t, "v1.0.0", tagList[0].Name)
}

func TestGitLabClient_V3_GetTagList_Error(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects/4/repository/tags" {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(``))
		}
	})
	defer ts.Close()

	//
	// test start
	//
	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	project := &Project{
		ID: 4,
	}

	tagList, err := client.GetTagList(project)

	assert.Error(t, err)
	assert.Nil(t, tagList)
}

func TestGitLabClient_V3_GetFile(t *testing.T) {
	ts := createTestGitLabAPIV3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/projects/4/repository/files" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/file/item_v3.json"))
		}
	})
	defer ts.Close()

	client, err := NewClient(ts.URL, testClientTokenValid)
	if err != nil {
		t.Fatal(err)
	}

	project := &Project{
		ID: 4,
	}

	fileContent, err := client.GetFile(project, "README.md", "master")
	assert.Nil(t, err)
	assert.Equal(t, []byte("Hello world"), fileContent)
}
