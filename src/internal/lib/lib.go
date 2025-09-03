// The lib package is the implementation of the core functionality of the raid CLI tool.
package lib

const (
	YAML_SEP = "---"
)

type Context struct {
	Profile Profile
}

var context *Context

func Compile() error {
	if context == nil {
		return ForceCompile()
	}
	return nil
}

func ForceCompile() error {
	profile, err := BuildProfile(GetProfile())
	if err != nil {
		return err
	}
	context = &Context{
		Profile: profile,
	}
	return nil
}
