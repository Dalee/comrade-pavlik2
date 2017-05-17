package gitlab

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestGitLabClient_V4_ProjectList(t *testing.T) {
	ts := createTestGitLabAPIV4(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/project/list_v4.json"))
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

func TestGitLabClient_V4_GetProjectById(t *testing.T) {
	ts := createTestGitLabAPIV4(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/4" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/project/item_v4.json"))
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

func TestGitLabClient_V4_GetTagList(t *testing.T) {
	ts := createTestGitLabAPIV4(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/4/repository/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write(getTestRawDataFromFile(t, "./test-data/tag/list_v4.json"))
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
