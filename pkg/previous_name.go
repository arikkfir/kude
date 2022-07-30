package kude

import (
	"fmt"
	"github.com/arikkfir/kyaml/pkg"
)

func GetResourcePreviousName(r *kyaml.RNode) (string, error) {
	value, err := r.GetAnnotation(PreviousNameAnnotationName)
	if err != nil {
		return "", fmt.Errorf("failed getting annotation: %w", err)
	} else {
		return value, nil
	}
}
