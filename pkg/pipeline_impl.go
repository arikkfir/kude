package kude

type pipelineImpl struct {
	APIVersion             string      `yaml:"apiVersion"`
	Kind                   string      `yaml:"kind"`
	pwd                    string      `yaml:"-"`
	Resources              []string    `yaml:"resources"`
	Steps                  []*stepImpl `yaml:"steps"`
	inlineBuiltinFunctions bool
}

func (p *pipelineImpl) GetAPIVersion() string  { return p.APIVersion }
func (p *pipelineImpl) GetKind() string        { return p.Kind }
func (p *pipelineImpl) GetDirectory() string   { return p.pwd }
func (p *pipelineImpl) GetResources() []string { return p.Resources }

func (p *pipelineImpl) GetSteps() []Step {
	steps := make([]Step, len(p.Steps))
	for i, step := range p.Steps {
		steps[i] = step
	}
	return steps
}

type stepImpl struct {
	ID         string                 `yaml:"id"`
	Name       string                 `yaml:"name"`
	Image      string                 `yaml:"image"`
	Entrypoint []string               `yaml:"entrypoint"`
	User       string                 `yaml:"user"`
	Workdir    string                 `yaml:"workdir"`
	Network    bool                   `yaml:"network"`
	Mounts     []string               `yaml:"mounts"`
	Config     map[string]interface{} `yaml:"config"`
}

func (s stepImpl) GetID() string                     { return s.ID }
func (s stepImpl) GetName() string                   { return s.Name }
func (s stepImpl) GetImage() string                  { return s.Image }
func (s stepImpl) GetEntrypoint() []string           { return s.Entrypoint }
func (s stepImpl) GetUser() string                   { return s.User }
func (s stepImpl) GetWorkdir() string                { return s.Workdir }
func (s stepImpl) GetNetwork() bool                  { return s.Network }
func (s stepImpl) GetMounts() []string               { return s.Mounts }
func (s stepImpl) GetConfig() map[string]interface{} { return s.Config }
