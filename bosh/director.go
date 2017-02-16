package bosh

import (
	"io"
	"fmt"

	"github.com/cloudfoundry/bosh-deployment-resource/concourse"

	boshcmd "github.com/cloudfoundry/bosh-cli/cmd"
	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
)

type DeployParams struct {
	Vars     map[string]interface{}
	NoRedact bool
	Cleanup  bool
}

type Director interface {
	Deploy(manifestBytes []byte, deployParams DeployParams) error
	DownloadManifest() ([]byte, error)
	UploadRelease(releaseURL string) error
	UploadStemcell(stemcellURL string) error
}

type BoshDirector struct {
	source             concourse.Source
	commandRunner      Runner
	out                io.Writer
}

func NewBoshDirector(source concourse.Source, commandRunner Runner, out io.Writer) BoshDirector {
	return BoshDirector{
		source:             source,
		commandRunner:      commandRunner,
		out:                out,
	}
}

func (d BoshDirector) Deploy(manifestBytes []byte, deployParams DeployParams) error {
	if deployParams.Cleanup {
		d.commandRunner.Execute(&boshcmd.CleanUpOpts{})
	}

	err := d.commandRunner.Execute(&boshcmd.DeployOpts{
		Args:     boshcmd.DeployArgs{Manifest: boshcmd.FileBytesArg{Bytes: manifestBytes}},
		NoRedact: deployParams.NoRedact,
		VarFlags: boshcmd.VarFlags{
			VarKVs: varKVsFromVars(deployParams.Vars),
		},
	})
	if err != nil {
		return fmt.Errorf("Could not deploy: %s\n", err)
	}

	return nil
}

func (d BoshDirector) DownloadManifest() ([]byte, error) {
	bytes, err := d.commandRunner.GetResult(&boshcmd.ManifestOpts{})

	if err != nil {
		return nil, fmt.Errorf("Could not get deployment manifest: %s\n", err)
	}

	return bytes, nil
}

func (d BoshDirector) UploadRelease(URL string) error {
	err := d.commandRunner.Execute(&boshcmd.UploadReleaseOpts{
		Args:     boshcmd.UploadReleaseArgs{URL: boshcmd.URLArg(URL)},
	})

	if err != nil {
		return fmt.Errorf("Could not upload release %s: %s\n", URL, err)
	}

	return nil
}

func (d BoshDirector) UploadStemcell(URL string) error {
	err := d.commandRunner.Execute(&boshcmd.UploadStemcellOpts{
		Args:     boshcmd.UploadStemcellArgs{URL: boshcmd.URLArg(URL)},
	})

	if err != nil {
		return fmt.Errorf("Could not upload stemcell %s: %s\n", URL, err)
	}

	return nil
}

func varKVsFromVars(vars map[string]interface{}) []boshtpl.VarKV {
	varKVs := []boshtpl.VarKV{}
	for k, v := range vars {
		varKVs = append(varKVs, boshtpl.VarKV{Name: k, Value: v})
	}
	return varKVs
}