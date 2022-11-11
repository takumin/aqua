package policy

import (
	"fmt"

	"github.com/aquaproj/aqua/pkg/config"
	"github.com/aquaproj/aqua/pkg/expr"
)

type ParamValidatePackage struct {
	Pkg          *config.Package
	PolicyConfig *ConfigYAML
}

func (pc *Checker) ValidatePackage(param *ParamValidatePackage) error {
	if param.PolicyConfig == nil {
		return nil
	}
	for _, policyPkg := range param.PolicyConfig.Packages {
		f, err := pc.matchPkg(param.Pkg, policyPkg)
		if err != nil {
			return err
		}
		if f {
			return nil
		}
	}
	return errUnAllowedPackage
}

func (pc *Checker) matchPkg(pkg *config.Package, policyPkg *Package) (bool, error) {
	if policyPkg.Name != "" && pkg.Package.Name != policyPkg.Name {
		return false, nil
	}
	if policyPkg.Version != "" {
		matched, err := expr.EvaluateVersionConstraints(policyPkg.Version, pkg.Package.Version)
		if err != nil {
			return false, fmt.Errorf("evaluate the version constraint of package: %w", err)
		}
		if !matched {
			return false, nil
		}
	}
	return pc.matchRegistry(pkg.Registry, policyPkg.Registry)
}
