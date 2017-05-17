package gitlab

type (
	Project struct {
		ID                int      `json:"id"`
		Name              string   `json:"name"`
		PathWithNamespace string   `json:"path_with_namespace"`
		SSHURL            string   `json:"ssh_url_to_repo"`
		HTTPURL           string   `json:"http_url_to_repo"`
		WWWURL            string   `json:"web_url"`
		TagList           []string `json:"tag_list"`
	}

	commitInlined struct {
		ID string `json:"id"`
	}

	Tag struct {
		Name   string        `json:"name"`
		Commit commitInlined `json:"commit"`
	}

	File struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
)
