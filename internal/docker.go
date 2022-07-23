package internal

import "github.com/docker/docker/api/types"

func IsImageWithLatestTag(image *types.ImageSummary) bool {
	for _, tag := range image.RepoTags {
		if tag == "latest" {
			return true
		}
	}
	return false
}
