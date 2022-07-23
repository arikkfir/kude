package kude

const (
	PipelineAPIVersion = "kude.kfirs.com/v1alpha2"
	PipelineKind       = "Pipeline"
)

type Pipeline interface {
	GetAPIVersion() string
	GetKind() string
	GetDirectory() string
	GetResources() []string
	GetSteps() []Step
}

type Step interface {
	GetID() string
	GetName() string
	GetImage() string
	GetEntrypoint() []string
	GetUser() string
	GetWorkdir() string
	GetNetwork() bool
	GetMounts() []string
	GetConfig() map[string]interface{}
}
