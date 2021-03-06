package app

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/pflag"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/config"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
)

func getOpenShiftConfig(configFile string) (map[string]interface{}, error) {
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := kyaml.ToJSON(configBytes)
	if err != nil {
		return nil, err
	}
	config := map[string]interface{}{}
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func applyOpenShiftConfigFlags(controllerManagerOptions *options.KubeControllerManagerOptions, controllerManager *config.Config, openshiftConfig map[string]interface{}) error {
	if err := applyOpenShiftConfigControllerArgs(controllerManagerOptions, openshiftConfig); err != nil {
		return err
	}
	if err := applyOpenShiftConfigDefaultProjectSelector(controllerManagerOptions, openshiftConfig); err != nil {
		return err
	}
	if err := applyOpenShiftConfigKubeDefaultProjectSelector(controllerManagerOptions, openshiftConfig); err != nil {
		return err
	}
	return controllerManagerOptions.ApplyTo(controllerManager, "kube-controller-manager")
}

func applyOpenShiftConfigDefaultProjectSelector(controllerManagerOptions *options.KubeControllerManagerOptions, openshiftConfig map[string]interface{}) error {
	projectConfig, ok := openshiftConfig["projectConfig"]
	if !ok {
		return nil
	}

	castProjectConfig := projectConfig.(map[string]interface{})
	defaultNodeSelector, ok := castProjectConfig["defaultNodeSelector"]
	if !ok {
		return nil
	}
	controllerManagerOptions.OpenShiftContext.OpenShiftDefaultProjectNodeSelector = defaultNodeSelector.(string)

	return nil
}

// this is an optimization.  It can be filled in later.  Looks like there are several special cases for this plugin upstream
// TODO find this
func applyOpenShiftConfigKubeDefaultProjectSelector(controllerManagerOptions *options.KubeControllerManagerOptions, openshiftConfig map[string]interface{}) error {
	controllerManagerOptions.OpenShiftContext.KubeDefaultProjectNodeSelector = ""
	return nil
}

func applyOpenShiftConfigControllerArgs(controllerManagerOptions *options.KubeControllerManagerOptions, openshiftConfig map[string]interface{}) error {
	var controllerArgs interface{}
	kubeMasterConfig, ok := openshiftConfig["kubernetesMasterConfig"]
	if !ok {
		controllerArgs, ok = openshiftConfig["extendedArguments"]
		if !ok || controllerArgs == nil {
			return nil
		}
	} else {
		castKubeMasterConfig := kubeMasterConfig.(map[string]interface{})
		controllerArgs, ok = castKubeMasterConfig["controllerArguments"]
		if !ok || controllerArgs == nil {
			controllerArgs, ok = openshiftConfig["extendedArguments"]
			if !ok || controllerArgs == nil {
				return nil
			}
		}
	}

	args := map[string][]string{}
	for key, value := range controllerArgs.(map[string]interface{}) {
		for _, arrayValue := range value.([]interface{}) {
			args[key] = append(args[key], arrayValue.(string))
		}
	}
	if err := resolveFlags(args, kubeControllerManagerAddFlags(controllerManagerOptions)); len(err) > 0 {
		return kerrors.NewAggregate(err)
	}
	return nil
}

// applyFlags stores the provided arguments onto a flag set, reporting any errors
// encountered during the process.
func applyFlags(args map[string][]string, flags *pflag.FlagSet) []error {
	var errs []error
	for key, value := range args {
		flag := flags.Lookup(key)
		if flag == nil {
			errs = append(errs, field.Invalid(field.NewPath("flag"), key, "is not a valid flag"))
			continue
		}
		for _, s := range value {
			if err := flag.Value.Set(s); err != nil {
				errs = append(errs, field.Invalid(field.NewPath(key), s, fmt.Sprintf("could not be set: %v", err)))
				break
			}
		}
	}
	return errs
}

func resolveFlags(args map[string][]string, fn func(*pflag.FlagSet)) []error {
	fs := pflag.NewFlagSet("extended", pflag.ContinueOnError)
	fn(fs)
	return applyFlags(args, fs)
}

func kubeControllerManagerAddFlags(options *options.KubeControllerManagerOptions) func(flags *pflag.FlagSet) {
	return func(flags *pflag.FlagSet) {
		options.AddFlags(flags, KnownControllers(), ControllersDisabledByDefault.List())
	}
}
