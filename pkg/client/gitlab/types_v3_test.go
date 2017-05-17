package gitlab

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnmarshalProjectV3(t *testing.T) {
	project := Project{}

	data := getTestRawDataFromFile(t, "./test-data/project/item_v3.json")
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, project.ID)
	assert.Equal(t, "Diaspora Client", project.Name)
	assert.Equal(t, "diaspora/diaspora-client", project.PathWithNamespace)
	assert.Equal(t, "git@example.com:diaspora/diaspora-client.git", project.SSHURL)
	assert.Equal(t, "http://example.com/diaspora/diaspora-client.git", project.HTTPURL)
	assert.Equal(t, "http://example.com/diaspora/diaspora-client", project.WWWURL)
	assert.Equal(t, []string{"example", "disapora client"}, project.TagList)
}

func TestUnmarshalTagV3(t *testing.T) {
	tag := Tag{}

	data := getTestRawDataFromFile(t, "./test-data/tag/item_v3.json")
	if err := json.Unmarshal(data, &tag); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "v1.0.0", tag.Name)
	assert.Equal(t, "48bfe316395305d02af3f4b8bb9dec62d8e4c567", tag.Commit.ID)
}
