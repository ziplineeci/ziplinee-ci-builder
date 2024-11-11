package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	contracts "github.com/ziplineeci/ziplinee-ci-contracts"
	crypt "github.com/ziplineeci/ziplinee-ci-crypt"
	manifest "github.com/ziplineeci/ziplinee-ci-manifest"
	foundation "github.com/ziplineeci/ziplinee-foundation"
)

// EnvvarHelper is the interface for getting, setting and retrieving ZIPLINEE_ environment variables
type EnvvarHelper interface {
	getCommandOutput(string, ...string) (string, error)
	SetZiplineeGlobalEnvvars() error
	SetZiplineeBuilderConfigEnvvars(builderConfig contracts.BuilderConfig) error
	setZiplineeEventEnvvars(events []manifest.ZiplineeEvent) error
	initGitSource() error
	initGitOwner() error
	initGitName() error
	initGitFullName() error
	initGitRevision() error
	initGitBranch() error
	initBuildDatetime() error
	initBuildStatus() error
	initLabels(manifest.ZiplineeManifest) error
	collectZiplineeEnvvars() map[string]string
	CollectZiplineeEnvvarsAndLabels(manifest.ZiplineeManifest) (map[string]string, error)
	CollectGlobalEnvvars(manifest.ZiplineeManifest) map[string]string
	UnsetZiplineeEnvvars()
	getZiplineeEnv(string) string
	setZiplineeEnv(string, string) error
	unsetZiplineeEnv(string) error
	getZiplineeEnvvarName(string) string
	OverrideEnvvars(...map[string]string) map[string]string
	decryptSecret(string, string) string
	decryptSecrets(map[string]string, string) map[string]string
	GetCiServer() string
	SetPipelineName(builderConfig contracts.BuilderConfig) error
	GetPipelineName() string
	GetWorkDir() string
	GetTempDir() string
	GetPodName() string
	GetPodUID() string
	GetPodNamespace() string
	GetPodNodeName() string
	makeDNSLabelSafe(string) string

	getGitOrigin() (string, error)
	getSourceFromOrigin(string) string
	getOwnerFromOrigin(string) string
	getNameFromOrigin(string) string
}

type envvarHelper struct {
	prefix       string
	ciServer     string
	workDir      string
	tempDir      string
	secretHelper crypt.SecretHelper
	obfuscator   Obfuscator
}

// NewEnvvarHelper returns a new EnvvarHelper
func NewEnvvarHelper(prefix string, secretHelper crypt.SecretHelper, obfuscator Obfuscator) EnvvarHelper {
	return &envvarHelper{
		prefix:       prefix,
		ciServer:     os.Getenv("ZIPLINEE_CI_SERVER"),
		workDir:      os.Getenv("ZIPLINEE_WORKDIR"),
		tempDir:      os.Getenv("ZIPLINEE_TEMPDIR"),
		secretHelper: secretHelper,
		obfuscator:   obfuscator,
	}
}

func (h *envvarHelper) getCommandOutput(name string, arg ...string) (string, error) {

	out, err := exec.Command(name, arg...).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func (h *envvarHelper) SetZiplineeGlobalEnvvars() (err error) {

	// initialize build datetime envvar
	err = h.initBuildDatetime()
	if err != nil {
		return err
	}

	// initialize build status envvar
	err = h.initBuildStatus()
	if err != nil {
		return err
	}

	// remaining envvars are only set for gocd agent runs
	if h.ciServer != "gocd" {
		return
	}

	// initialize git source envvar
	err = h.initGitSource()
	if err != nil {
		return err
	}

	// initialize git owner envvar
	err = h.initGitOwner()
	if err != nil {
		return err
	}

	// initialize git name envvar
	err = h.initGitName()
	if err != nil {
		return err
	}

	// initialize git full name envvar
	err = h.initGitFullName()
	if err != nil {
		return err
	}

	// initialize git revision envvar
	err = h.initGitRevision()
	if err != nil {
		return err
	}

	// initialize git branch envvar
	err = h.initGitBranch()
	if err != nil {
		return err
	}

	return
}

func (h *envvarHelper) SetZiplineeBuilderConfigEnvvars(builderConfig contracts.BuilderConfig) (err error) {
	// set envvars that can be used by any container
	err = h.setZiplineeEnv("ZIPLINEE_GIT_SOURCE", builderConfig.Git.RepoSource)
	if err != nil {
		return
	}
	err = h.setZiplineeEnv("ZIPLINEE_GIT_OWNER", builderConfig.Git.RepoOwner)
	if err != nil {
		return
	}
	err = h.setZiplineeEnv("ZIPLINEE_GIT_NAME", builderConfig.Git.RepoName)
	if err != nil {
		return
	}
	err = h.setZiplineeEnv("ZIPLINEE_GIT_FULLNAME", fmt.Sprintf("%v/%v", builderConfig.Git.RepoOwner, builderConfig.Git.RepoName))
	if err != nil {
		return
	}
	err = h.setZiplineeEnv("ZIPLINEE_GIT_BRANCH", builderConfig.Git.RepoBranch)
	if err != nil {
		return
	}
	// set dns safe version
	err = h.setZiplineeEnv("ZIPLINEE_GIT_BRANCH_DNS_SAFE", h.makeDNSLabelSafe(builderConfig.Git.RepoBranch))
	if err != nil {
		return
	}
	err = h.setZiplineeEnv("ZIPLINEE_GIT_REVISION", builderConfig.Git.RepoRevision)
	if err != nil {
		return
	}
	err = h.setZiplineeEnv("ZIPLINEE_BUILD_VERSION", builderConfig.Version.Version)
	if err != nil {
		return
	}
	if builderConfig.Version != nil && builderConfig.Version.Major != nil {
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_VERSION_MAJOR", strconv.Itoa(*builderConfig.Version.Major))
		if err != nil {
			return
		}
	}
	if builderConfig.Version != nil && builderConfig.Version.Minor != nil {
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_VERSION_MINOR", strconv.Itoa(*builderConfig.Version.Minor))
		if err != nil {
			return
		}
	}
	if builderConfig.Version != nil && builderConfig.Version.AutoIncrement != nil {
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_VERSION_PATCH", strconv.Itoa(*builderConfig.Version.AutoIncrement))
		if err != nil {
			return
		}
	}
	if builderConfig.Version != nil && builderConfig.Version.Label != nil {
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_VERSION_LABEL", *builderConfig.Version.Label)
		if err != nil {
			return
		}
	}

	// set counters to enable release locking for older revisions inside extensions
	if builderConfig.Version != nil {
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_CURRENT_COUNTER", strconv.Itoa(builderConfig.Version.CurrentCounter))
		if err != nil {
			return
		}
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_MAX_COUNTER", strconv.Itoa(builderConfig.Version.MaxCounter))
		if err != nil {
			return
		}
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_MAX_COUNTER_CURRENT_BRANCH", strconv.Itoa(builderConfig.Version.MaxCounterCurrentBranch))
		if err != nil {
			return
		}
	}

	if builderConfig.Build != nil {
		// set ZIPLINEE_BUILD_ID for backwards compatibility with extensions/github-status and extensions/bitbucket-status and extensions/slack-build-status
		err = h.setZiplineeEnv("ZIPLINEE_BUILD_ID", builderConfig.Build.ID)
		if err != nil {
			return
		}
	}
	if builderConfig.Release != nil {
		err = h.setZiplineeEnv("ZIPLINEE_RELEASE_NAME", builderConfig.Release.Name)
		if err != nil {
			return
		}
		err = h.setZiplineeEnv("ZIPLINEE_RELEASE_ACTION", builderConfig.Release.Action)
		if err != nil {
			return
		}
		// set ZIPLINEE_RELEASE_ID for backwards compatibility with extensions/slack-build-status
		err = h.setZiplineeEnv("ZIPLINEE_RELEASE_ID", builderConfig.Release.ID)
		if err != nil {
			return
		}

		triggeredBy := ""
		if len(builderConfig.Events) > 0 {
			for _, e := range builderConfig.Events {
				if e.Manual != nil {
					triggeredBy = e.Manual.UserID
				}
			}
		}
		err = h.setZiplineeEnv("ZIPLINEE_RELEASE_TRIGGERED_BY", triggeredBy)
		if err != nil {
			return
		}
	}
	if builderConfig.Bot != nil {
		err = h.setZiplineeEnv("ZIPLINEE_BOT_NAME", builderConfig.Bot.Name)
		if err != nil {
			return
		}
		err = h.setZiplineeEnv("ZIPLINEE_BOT_ID", builderConfig.Bot.ID)
		if err != nil {
			return
		}
	}

	// set ZIPLINEE_CI_SERVER_BASE_URL for backwards compatibility with extensions/github-status and extensions/bitbucket-status and extensions/slack-build-status
	if builderConfig.CIServer != nil {
		err = h.setZiplineeEnv("ZIPLINEE_CI_SERVER_BASE_URL", builderConfig.CIServer.BaseURL)
		if err != nil {
			return
		}
	}

	return h.setZiplineeEventEnvvars(builderConfig.Events)
}

func (h *envvarHelper) setZiplineeEventEnvvars(events []manifest.ZiplineeEvent) (err error) {

	for _, e := range events {
		triggerFields := reflect.TypeOf(e)
		triggerValues := reflect.ValueOf(e)

		for i := 0; i < triggerFields.NumField(); i++ {

			triggerField := triggerFields.Field(i).Name
			triggerValue := triggerValues.Field(i)

			if triggerValue.Kind() != reflect.Ptr || triggerValue.IsNil() {
				continue
			}

			// dereference the pointer
			derefencedPointerValue := reflect.Indirect(triggerValue)

			triggerPropertyFields := derefencedPointerValue.Type()
			triggerPropertyValues := derefencedPointerValue

			for j := 0; j < triggerPropertyFields.NumField(); j++ {

				triggerPropertyField := triggerPropertyFields.Field(j).Name
				triggerPropertyValue := triggerPropertyValues.Field(j)

				envvarName := "ZIPLINEE_TRIGGER_" + foundation.ToUpperSnakeCase(triggerField) + "_" + foundation.ToUpperSnakeCase(triggerPropertyField)
				envvarValue := ""

				switch triggerPropertyValue.Kind() {
				case reflect.String:
					envvarValue = triggerPropertyValue.String()
				default:
					jsonValue, _ := json.Marshal(triggerPropertyValue.Interface())
					envvarValue = string(jsonValue)

					envvarValue = strings.Trim(envvarValue, "\"")
				}

				if e.Fired {
					err = h.setZiplineeEnv(envvarName, envvarValue)
					if err != nil {
						return
					}
				}

				if e.Name != "" {
					// set envvar for named trigger/event, in order to have upstream pipelines and release when they're not fired as well
					envvarName := "ZIPLINEE_TRIGGER_" + foundation.ToUpperSnakeCase(e.Name) + "_" + foundation.ToUpperSnakeCase(triggerPropertyField)
					err = h.setZiplineeEnv(envvarName, envvarValue)
					if err != nil {
						return
					}
				}
			}
		}
	}

	return nil
}

func (h *envvarHelper) getGitOrigin() (string, error) {
	return h.getCommandOutput("git", "config", "--get", "remote.origin.url")
}

func (h *envvarHelper) initGitSource() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_GIT_SOURCE") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		source := h.getSourceFromOrigin(origin)
		return h.setZiplineeEnv("ZIPLINEE_GIT_SOURCE", source)
	}
	return
}

func (h *envvarHelper) initGitOwner() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_GIT_OWNER") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		owner := h.getOwnerFromOrigin(origin)
		return h.setZiplineeEnv("ZIPLINEE_GIT_OWNER", owner)
	}
	return
}

func (h *envvarHelper) initGitName() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_GIT_NAME") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		name := h.getNameFromOrigin(origin)
		return h.setZiplineeEnv("ZIPLINEE_GIT_NAME", name)
	}
	return
}

func (h *envvarHelper) initGitFullName() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_GIT_FULLNAME") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		owner := h.getOwnerFromOrigin(origin)
		name := h.getNameFromOrigin(origin)
		return h.setZiplineeEnv("ZIPLINEE_GIT_FULLNAME", fmt.Sprintf("%v/%v", owner, name))
	}
	return
}

func (h *envvarHelper) SetPipelineName(builderConfig contracts.BuilderConfig) (err error) {

	if builderConfig.Git == nil {
		err = h.initGitSource()
		if err != nil {
			return
		}

		err = h.initGitOwner()
		if err != nil {
			return
		}

		err = h.initGitName()
		if err != nil {
			return
		}

		return nil
	}

	err = h.setZiplineeEnv("ZIPLINEE_GIT_SOURCE", builderConfig.Git.RepoSource)
	if err != nil {
		return
	}

	err = h.setZiplineeEnv("ZIPLINEE_GIT_OWNER", builderConfig.Git.RepoOwner)
	if err != nil {
		return
	}

	err = h.setZiplineeEnv("ZIPLINEE_GIT_NAME", builderConfig.Git.RepoName)
	if err != nil {
		return
	}

	return nil
}

func (h *envvarHelper) GetPipelineName() string {

	source := h.getZiplineeEnv("ZIPLINEE_GIT_SOURCE")
	owner := h.getZiplineeEnv("ZIPLINEE_GIT_OWNER")
	name := h.getZiplineeEnv("ZIPLINEE_GIT_NAME")

	if source == "" || owner == "" || name == "" {
		log.Fatal().Msg("Git environment variables have not been set yet, cannot resolve pipeline name")
	}

	return fmt.Sprintf("%v/%v/%v", source, owner, name)
}

func (h *envvarHelper) getSourceFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[2]
}

func (h *envvarHelper) getOwnerFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[4]
}

func (h *envvarHelper) getNameFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[5]
}

func (h *envvarHelper) initGitRevision() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_GIT_REVISION") == "" {
		revision, err := h.getCommandOutput("git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		return h.setZiplineeEnv("ZIPLINEE_GIT_REVISION", revision)
	}
	return
}

func (h *envvarHelper) initGitBranch() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_GIT_BRANCH") == "" {
		branch, err := h.getCommandOutput("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}
		return h.setZiplineeEnv("ZIPLINEE_GIT_BRANCH", branch)
	}
	return
}

func (h *envvarHelper) initBuildDatetime() (err error) {
	if h.getZiplineeEnv("ZIPLINEE_BUILD_DATETIME") == "" {
		return h.setZiplineeEnv("ZIPLINEE_BUILD_DATETIME", time.Now().UTC().Format(time.RFC3339))
	}
	return
}

func (h *envvarHelper) initBuildStatus() (err error) {
	return h.setZiplineeEnv("ZIPLINEE_BUILD_STATUS", "succeeded")
}

func (h *envvarHelper) initLabels(m manifest.ZiplineeManifest) (err error) {

	// set labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {
			envvarName := "ZIPLINEE_LABEL_" + foundation.ToUpperSnakeCase(key)
			err = h.setZiplineeEnv(envvarName, value)
			if err != nil {
				return
			}
		}
	}

	return
}

func (h *envvarHelper) CollectZiplineeEnvvarsAndLabels(m manifest.ZiplineeManifest) (envvars map[string]string, err error) {

	// set labels as envvars
	err = h.initLabels(m)
	if err != nil {
		return
	}

	// return all envvars starting with ZIPLINEE_
	return h.collectZiplineeEnvvars(), nil
}

func (h *envvarHelper) collectZiplineeEnvvars() (envvars map[string]string) {

	// return all envvars starting with ZIPLINEE_
	envvars = map[string]string{}

	for _, e := range os.Environ() {
		kvPair := strings.SplitN(e, "=", 2)

		if len(kvPair) == 2 {
			envvarName := kvPair[0]
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, h.prefix) {
				envvars[envvarName] = envvarValue
			}
		}
	}

	return
}

func (h *envvarHelper) CollectGlobalEnvvars(m manifest.ZiplineeManifest) (envvars map[string]string) {

	envvars = map[string]string{}

	if m.GlobalEnvVars != nil {
		envvars = m.GlobalEnvVars
	}

	return
}

// only to be used from unit tests
func (h *envvarHelper) UnsetZiplineeEnvvars() {

	envvarsToUnset := h.collectZiplineeEnvvars()
	for key := range envvarsToUnset {
		err := h.unsetZiplineeEnv(key)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed unseeting envvar %v", key)
		}
	}
}

func (h *envvarHelper) getZiplineeEnv(key string) string {

	key = h.getZiplineeEnvvarName(key)

	if strings.HasPrefix(key, h.prefix) {
		return os.Getenv(key)
	}

	return fmt.Sprintf("${%v}", key)
}

func (h *envvarHelper) setZiplineeEnv(key, value string) error {

	key = h.getZiplineeEnvvarName(key)

	err := os.Setenv(key, value)
	if err != nil {
		return err
	}

	return nil
}

func (h *envvarHelper) unsetZiplineeEnv(key string) error {

	key = h.getZiplineeEnvvarName(key)

	return os.Unsetenv(key)
}

func (h *envvarHelper) getZiplineeEnvvarName(key string) string {
	return strings.Replace(key, "ZIPLINEE_", h.prefix, -1)
}

func (h *envvarHelper) OverrideEnvvars(envvarMaps ...map[string]string) (envvars map[string]string) {

	envvars = make(map[string]string)
	for _, envvarMap := range envvarMaps {
		for k, v := range envvarMap {
			envvars[k] = v
		}
	}

	return
}

func (h *envvarHelper) decryptSecret(encryptedValue, pipeline string) (decryptedValue string) {

	decryptedValue, err := h.secretHelper.DecryptAllEnvelopes(encryptedValue, pipeline)

	if err != nil {
		log.Warn().Err(err).Msg("Failed decrypting secret")
		return encryptedValue
	}

	return
}

func (h *envvarHelper) decryptSecrets(encryptedEnvvars map[string]string, pipeline string) (envvars map[string]string) {

	if len(encryptedEnvvars) == 0 {
		return encryptedEnvvars
	}

	envvars = make(map[string]string)
	for k, v := range encryptedEnvvars {
		envvars[k] = h.decryptSecret(v, pipeline)
	}

	return
}

func (h *envvarHelper) GetCiServer() string {
	return h.ciServer
}

func (h *envvarHelper) GetWorkDir() string {
	return h.workDir
}

func (h *envvarHelper) GetTempDir() string {
	return h.tempDir
}

func (h *envvarHelper) GetPodName() string {
	return os.Getenv("POD_NAME")
}

func (h *envvarHelper) GetPodUID() string {
	return os.Getenv("POD_UID")
}

func (h *envvarHelper) GetPodNamespace() string {
	return os.Getenv("POD_NAMESPACE")
}

func (h *envvarHelper) GetPodNodeName() string {
	return os.Getenv("POD_NODE_NAME")
}

func (h *envvarHelper) makeDNSLabelSafe(value string) string {
	// in order for the label to be used as a dns label (part between dots) it should only use
	// lowercase letters, digits and hyphens and have a max length of 63 characters;
	// also it should start with a letter and not end in a hyphen

	// ensure the label is lowercase
	value = strings.ToLower(value)

	// replace all invalid characters with a hyphen
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	value = reg.ReplaceAllString(value, "-")

	// replace double hyphens with a single one
	value = strings.Replace(value, "--", "-", -1)

	// trim hyphens from start and end
	value = strings.Trim(value, "-")

	// ensure it starts with a letter, not a digit or hyphen
	reg = regexp.MustCompile(`^[0-9-]+`)
	value = reg.ReplaceAllString(value, "")

	if len(value) > 63 {
		value = value[:63]
	}

	// trim hyphens from start and end
	value = strings.Trim(value, "-")

	return value
}
