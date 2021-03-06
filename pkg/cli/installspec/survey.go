package installspec

import (
	"fmt"

	v1 "github.com/solo-io/service-mesh-hub/api/v1"
	"github.com/solo-io/service-mesh-hub/pkg/registry"
	"github.com/solo-io/service-mesh-hub/pkg/render"
	"github.com/solo-io/service-mesh-hub/pkg/render/util"
	"gopkg.in/AlecAivazis/survey.v1"
)

func GetInstallSpec(reader registry.SpecReader, installNamespace string) (*InstallSpec, error) {
	spec, err := selectApplication(reader)
	if err != nil {
		return nil, err
	}
	version, err := selectVersion(spec)
	if err != nil {
		return nil, err
	}
	values, err := GetValuesInputs(spec, version, installNamespace)
	if err != nil {
		return nil, err
	}

	return &InstallSpec{
		Values:  *values,
		Version: version,
	}, nil
}

func GetValuesInputs(spec *v1.ApplicationSpec, version *v1.VersionedApplicationSpec, installNamespace string) (*render.ValuesInputs, error) {
	values := render.ValuesInputs{
		Name:              spec.Name,
		InstallNamespace:  installNamespace,
		SpecDefinedValues: version.ValuesYaml,
		Params:            make(map[string]string),
	}

	if err := selectParams(version.GetParameters(), values.Params); err != nil {
		return nil, err
	}

	flavor, err := selectFlavor(version)
	if err != nil {
		return nil, err
	}
	values.Flavor = flavor
	if err = selectParams(flavor.GetParameters(), values.Params); err != nil {
		return nil, err
	}

	if values.Layers, err = selectLayerInputList(flavor); err != nil {
		return nil, err
	}

	for _, layer := range flavor.GetCustomizationLayers() {
		for _, layerInput := range values.Layers {
			if layer.Id == layerInput.LayerId {
				for _, option := range layer.Options {
					if option.Id == layerInput.OptionId {
						if err := selectParams(option.GetParameters(), values.Params); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	return &values, nil
}

func selectApplication(reader registry.SpecReader) (*v1.ApplicationSpec, error) {
	specs, err := reader.GetSpecs()
	if err != nil {
		return nil, err
	}
	var names []string
	nameToSpec := make(map[string]*v1.ApplicationSpec)
	for _, spec := range specs {
		nameToSpec[spec.Name] = spec
		names = append(names, spec.Name)
	}
	specName := ""
	prompt := &survey.Select{
		Options:  names,
		Message:  "What application to install?",
		PageSize: 10,
	}
	err = survey.AskOne(prompt, &specName, survey.Required)
	if err != nil {
		return nil, err
	}
	return nameToSpec[specName], nil
}

func selectVersion(spec *v1.ApplicationSpec) (*v1.VersionedApplicationSpec, error) {
	var versions []string
	versionToSpec := make(map[string]*v1.VersionedApplicationSpec)
	for _, versionedSpec := range spec.Versions {
		versionToSpec[versionedSpec.Version] = versionedSpec
		versions = append(versions, versionedSpec.Version)
	}
	specVersion := ""
	prompt := &survey.Select{
		Options:  versions,
		Message:  "What version to install?",
		PageSize: 10,
	}
	err := survey.AskOne(prompt, &specVersion, survey.Required)
	if err != nil {
		return nil, err
	}
	return versionToSpec[specVersion], nil
}

func selectFlavor(spec *v1.VersionedApplicationSpec) (*v1.Flavor, error) {
	var flavors []string
	nameToFlavor := make(map[string]*v1.Flavor)
	for _, flavor := range spec.GetFlavors() {
		nameToFlavor[flavor.Name] = flavor
		flavors = append(flavors, flavor.Name)
	}
	flavor := ""
	prompt := &survey.Select{
		Options:  flavors,
		Message:  "What flavor to install?",
		PageSize: 10,
	}
	err := survey.AskOne(prompt, &flavor, survey.Required)
	if err != nil {
		return nil, err
	}
	return nameToFlavor[flavor], nil
}

func selectLayerInputList(flavor *v1.Flavor) ([]render.LayerInput, error) {
	layerInputList := make([]render.LayerInput, 0, len(flavor.GetCustomizationLayers()))
	for _, layer := range flavor.GetCustomizationLayers() {
		option, err := selectLayerOption(layer)
		if err != nil {
			return nil, err
		}

		// handle optional layers
		if option == nil {
			continue
		}

		layerInputList = append(layerInputList, render.LayerInput{
			LayerId:  layer.Id,
			OptionId: option.Id,
		})
	}
	return layerInputList, nil
}

func selectLayerOption(layer *v1.Layer) (*v1.LayerOption, error) {
	layerOptions := make([]string, 0, len(layer.Options))
	displayNameToLayerOption := make(map[string]*v1.LayerOption, len(layerOptions))
	for _, option := range layer.GetOptions() {
		layerOptions = append(layerOptions, option.DisplayName)
		displayNameToLayerOption[option.DisplayName] = option
	}

	var v survey.Validator
	if layer.Optional {
		layerOptions = append(layerOptions, "< skip >")
		displayNameToLayerOption["< skip >"] = nil
	} else {
		v = survey.Required
	}

	option := ""
	prompt := &survey.Select{
		Options:  layerOptions,
		Message:  fmt.Sprintf("Select an option for layer %v.", layer.DisplayName),
		PageSize: 10,
	}

	if err := survey.AskOne(prompt, &option, v); err != nil {
		return nil, err
	}
	return displayNameToLayerOption[option], nil
}

func selectParams(specs []*v1.Parameter, dest map[string]string) error {
	for _, spec := range specs {
		val, err := selectParam(spec)
		if err != nil {
			return err
		}
		dest[spec.Name] = val
	}
	return nil
}

func selectParam(spec *v1.Parameter) (string, error) {
	d, err := util.ParamValueToString(spec.Default, util.PlainTextSecretGetter)
	if err != nil {
		return "", err
	}
	prompt := &survey.Input{
		Default: d,
		Message: fmt.Sprintf("[%s] %s", spec.Description, spec.Name),
	}
	input := ""
	err = survey.AskOne(prompt, &input, nil)
	return input, err
}
