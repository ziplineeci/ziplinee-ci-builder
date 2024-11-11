package builder

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	crypt "github.com/ziplineeci/ziplinee-ci-crypt"
	manifest "github.com/ziplineeci/ziplinee-ci-manifest"
)

func TestOverrideEnvvars(t *testing.T) {

	t.Run("CombinesAllEnvvarsFromPassedMaps", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		outerMap := map[string]string{
			"ENVVAR1": "value1",
		}
		innerMap := map[string]string{
			"ENVVAR2": "value2",
		}

		// act
		envvars := envvarHelper.OverrideEnvvars(outerMap, innerMap)

		assert.Equal(t, 2, len(envvars))
	})

	t.Run("OverridesEnvarFromFirstMapWithSecondMap", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		outerMap := map[string]string{
			"ENVVAR1": "value1",
		}
		innerMap := map[string]string{
			"ENVVAR1": "value2",
		}

		// act
		envvars := envvarHelper.OverrideEnvvars(outerMap, innerMap)

		assert.Equal(t, 1, len(envvars))
		assert.Equal(t, "value2", envvars["ENVVAR1"])
	})
}

func TestGetZiplineeEnvvarName(t *testing.T) {

	t.Run("ReturnsKeyNameWithZiplineeUnderscoreReplacedWithZiplineeEnvvarPrefixValue", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		key := envvarHelper.getZiplineeEnvvarName("ZIPLINEE_KEY")

		assert.Equal(t, "TESTPREFIX_KEY", key)
	})
}

func TestCollectZiplineeEnvvarsAndLabels(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithZiplinee", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{}

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsOneLabelAsZiplineeLabelLabel", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{Labels: map[string]string{"app": "ziplinee-ci-builder"}}

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "ziplinee-ci-builder", envvars["TESTPREFIX_LABEL_APP"])
	})

	t.Run("ReturnsOneLabelAsZiplineeLabelLabelWithSnakeCasing", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{Labels: map[string]string{"owningTeam": "ziplinee-ci-team"}}

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		log.Debug().Interface("envvars", envvars).Msg("")
		_, exists := envvars["TESTPREFIX_LABEL_OWNING_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "ziplinee-ci-team", envvars["TESTPREFIX_LABEL_OWNING_TEAM"])
	})

	t.Run("ReturnsTwoLabelsAsZiplineeLabelLabel", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{Labels: map[string]string{"app": "ziplinee-ci-builder", "team": "ziplinee-ci-team"}}

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "ziplinee-ci-builder", envvars["TESTPREFIX_LABEL_APP"])

		_, exists = envvars["TESTPREFIX_LABEL_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "ziplinee-ci-team", envvars["TESTPREFIX_LABEL_TEAM"])
	})

	t.Run("ReturnsOneEnvvarStartingWithZiplinee", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsOneEnvvarStartingWithZiplineeIfValueContainsIsSymbol", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{}
		os.Setenv("TESTPREFIX_VERSION", "b=c")

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "b=c", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsTwoEnvvarsStartingWithZiplinee", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")
		os.Setenv("TESTPREFIX_GIT_REPOSITORY", "git@github.com:ziplineeci/ziplinee-ci-builder.git")

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		_, exists = envvars["TESTPREFIX_GIT_REPOSITORY"]
		assert.True(t, exists)
		assert.Equal(t, "git@github.com:ziplineeci/ziplinee-ci-builder.git", envvars["TESTPREFIX_GIT_REPOSITORY"])
	})

	t.Run("ReturnsMixOfLabelsAndEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{Labels: map[string]string{"app": "ziplinee-ci-builder"}}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars, err := envvarHelper.CollectZiplineeEnvvarsAndLabels(manifest)

		assert.Nil(t, err)
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		_, exists = envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "ziplinee-ci-builder", envvars["TESTPREFIX_LABEL_APP"])
	})
}

func TestCollectGlobalEnvvars(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoGlobalEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{}

		// act
		envvars := envvarHelper.CollectGlobalEnvvars(manifest)

		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsGlobalEnvvarsIfManifestHasGlobalEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		manifest := manifest.ZiplineeManifest{GlobalEnvVars: map[string]string{"VAR_A": "Greetings", "VAR_B": "World"}}

		// act
		envvars := envvarHelper.CollectGlobalEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "Greetings", envvars["VAR_A"])
		assert.Equal(t, "World", envvars["VAR_B"])
	})
}

func TestGetZiplineeEnv(t *testing.T) {

	t.Run("ReturnsEnvironmentVariableValueIfItStartsWithZiplineeUnderscore", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		os.Setenv("TESTPREFIX_BUILD_STATUS", "succeeded")

		// act
		result := envvarHelper.getZiplineeEnv("TESTPREFIX_BUILD_STATUS")

		assert.Equal(t, "succeeded", result)
	})

	t.Run("ReturnsEnvironmentVariablePlaceholderIfItDoesNotStartWithZiplineeUnderscore", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		os.Setenv("HOME", "/root")

		// act
		result := envvarHelper.getZiplineeEnv("HOME")

		assert.Equal(t, "${HOME}", result)

	})
}

func TestDecryptSecret(t *testing.T) {

	t.Run("ReturnsOriginalValueIfDoesNotMatchZiplineeSecret", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		result := envvarHelper.decryptSecret("not a secret", "github.com/ziplineeci/ziplinee-ci-builder")

		assert.Equal(t, "not a secret", result)
	})

	t.Run("ReturnsUnencryptedValueIfMatchesZiplineeSecret", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "ziplinee.secret(uZmMgyMrf01fNsGb.R1JW-94cLgQi_CTZ9IQZy_kPpWkp2J5BfH26_TFHNduX)"

		// act
		result := envvarHelper.decryptSecret(value, "github.com/ziplineeci/ziplinee-ci-builder")

		assert.Equal(t, "this is my secret", result)
	})

	t.Run("ReturnsUnencryptedValueIfMatchesZiplineeSecretWithDoubleEqualSign", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "ziplinee.secret(JL-gvON80FpfLXqE.j4cl0_7BOpKbSlAmKGTERmL4nXd53nC7)"

		// act
		result := envvarHelper.decryptSecret(value, "github.com/ziplineeci/ziplinee-ci-builder")

		assert.Equal(t, "ziplinee", result)
	})
}

func TestDecryptSecrets(t *testing.T) {

	t.Run("ReturnsOriginalValueIfDoesNotMatchZiplineeSecret", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		envvars := map[string]string{
			"SOME_PLAIN_ENVVAR": "not a secret",
		}

		// act
		result := envvarHelper.decryptSecrets(envvars, "github.com/ziplineeci/ziplinee-ci-builder")

		assert.Equal(t, 1, len(result))
		assert.Equal(t, "not a secret", result["SOME_PLAIN_ENVVAR"])
	})

	t.Run("ReturnsUnencryptedValueIfMatchesZiplineeSecret", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		envvars := map[string]string{
			"SOME_SECRET": "ziplinee.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)",
		}

		// act
		result := envvarHelper.decryptSecrets(envvars, "github.com/ziplineeci/ziplinee-ci-builder")

		assert.Equal(t, 1, len(result))
		assert.Equal(t, "this is my secret", result["SOME_SECRET"])
	})
}

func TestGetSourceFromOrigin(t *testing.T) {

	t.Run("ReturnsHostFromHttpsUrl", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		source := envvarHelper.getSourceFromOrigin("https://github.com/ziplineeci/ziplinee-gcloud-mig-scaler.git")

		assert.Equal(t, "github.com", source)
	})

	t.Run("ReturnsHostFromGitUrl", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		source := envvarHelper.getSourceFromOrigin("git@github.com:ziplineeci/ziplinee-ci-builder.git")

		assert.Equal(t, "github.com", source)
	})
}

func TestGetOwnerFromOrigin(t *testing.T) {

	t.Run("ReturnsOwnerFromHttpsUrl", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		owner := envvarHelper.getOwnerFromOrigin("https://github.com/ziplineeci/ziplinee-gcloud-mig-scaler.git")

		assert.Equal(t, "ziplineeci", owner)
	})

	t.Run("ReturnsOwnerFromGitUrl", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		owner := envvarHelper.getOwnerFromOrigin("git@github.com:ziplineeci/ziplinee-ci-builder.git")

		assert.Equal(t, "ziplineeci", owner)
	})
}

func TestGetNameFromOrigin(t *testing.T) {

	t.Run("ReturnsNameFromHttpsUrl", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		name := envvarHelper.getNameFromOrigin("https://github.com/ziplineeci/ziplinee-gcloud-mig-scaler.git")

		assert.Equal(t, "ziplinee-gcloud-mig-scaler", name)
	})

	t.Run("ReturnsNameFromGitUrl", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()

		// act
		name := envvarHelper.getNameFromOrigin("git@github.com:ziplineeci/ziplinee-ci-builder.git")

		assert.Equal(t, "ziplinee-ci-builder", name)
	})
}

func TestMakeDNSLabelSafe(t *testing.T) {

	t.Run("ReturnsValueIfAlreadySafeForDNSLabel", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "dns-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsAllLowercaseIfHasUppercase", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "DNS-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsAllLowercaseIfHasUppercase", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "DNS-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsCharactersOtherThanLettersDigitsOrHyphensAsHyphens", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "dns-safe.value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsMultipleHyphensAsSingleHyphen", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "dns-safe--value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsStartingWithLetter", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "10-dns-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsWithHyphensTrimmed", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "-dns-safe-value-"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsTruncatedTo63Characters", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk", safeValue)
	})

	t.Run("ReturnsTruncatedTo63CharactersWithHyphensTrimmed", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		value := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghij-l"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghij", safeValue)
	})
}

func TestSetZiplineeEventEnvvars(t *testing.T) {

	t.Run("ReturnsPipelineEventPropertiesAsEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		event := manifest.ZiplineeEvent{
			Fired: true,
			Pipeline: &manifest.ZiplineePipelineEvent{
				BuildVersion: "1.0.50-some-branch",
				RepoSource:   "github.com",
				RepoOwner:    "ziplineeci",
				RepoName:     "ziplinee-ci-api",
				Branch:       "main",
				Status:       "succeeded",
				Event:        "finished",
			},
		}

		// act
		err := envvarHelper.setZiplineeEventEnvvars([]manifest.ZiplineeEvent{event})

		assert.Nil(t, err)
		envvars := envvarHelper.collectZiplineeEnvvars()
		assert.Equal(t, 7, len(envvars))
		assert.Equal(t, "1.0.50-some-branch", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_BUILD_VERSION"))
		assert.Equal(t, "github.com", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_REPO_SOURCE"))
		assert.Equal(t, "ziplineeci", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_REPO_OWNER"))
		assert.Equal(t, "ziplinee-ci-api", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_REPO_NAME"))
		assert.Equal(t, "main", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_BRANCH"))
		assert.Equal(t, "succeeded", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_STATUS"))
		assert.Equal(t, "finished", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_PIPELINE_EVENT"))
	})

	t.Run("ReturnsNamedPipelineEventPropertiesAsEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		event := manifest.ZiplineeEvent{
			Name:  "upstream",
			Fired: false,
			Pipeline: &manifest.ZiplineePipelineEvent{
				BuildVersion: "1.0.50-some-branch",
				RepoSource:   "github.com",
				RepoOwner:    "ziplineeci",
				RepoName:     "ziplinee-ci-api",
				Branch:       "main",
				Status:       "succeeded",
				Event:        "finished",
			},
		}

		// act
		err := envvarHelper.setZiplineeEventEnvvars([]manifest.ZiplineeEvent{event})

		assert.Nil(t, err)
		_ = envvarHelper.collectZiplineeEnvvars()
		assert.Equal(t, "1.0.50-some-branch", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_BUILD_VERSION"))
		assert.Equal(t, "github.com", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_REPO_SOURCE"))
		assert.Equal(t, "ziplineeci", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_REPO_OWNER"))
		assert.Equal(t, "ziplinee-ci-api", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_REPO_NAME"))
		assert.Equal(t, "main", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_BRANCH"))
		assert.Equal(t, "succeeded", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_STATUS"))
		assert.Equal(t, "finished", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_UPSTREAM_EVENT"))
	})

	t.Run("ReturnsReleaseEventPropertiesAsEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		event := manifest.ZiplineeEvent{
			Fired: true,
			Release: &manifest.ZiplineeReleaseEvent{
				ReleaseVersion: "1.0.50-some-branch",
				RepoSource:     "github.com",
				RepoOwner:      "ziplineeci",
				RepoName:       "ziplinee-ci-api",
				Target:         "development",
				Status:         "succeeded",
				Event:          "finished",
			},
		}

		// act
		err := envvarHelper.setZiplineeEventEnvvars([]manifest.ZiplineeEvent{event})

		assert.Nil(t, err)
		envvars := envvarHelper.collectZiplineeEnvvars()
		assert.Equal(t, 7, len(envvars))
		assert.Equal(t, "1.0.50-some-branch", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_RELEASE_VERSION"))
		assert.Equal(t, "github.com", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_REPO_SOURCE"))
		assert.Equal(t, "ziplineeci", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_REPO_OWNER"))
		assert.Equal(t, "ziplinee-ci-api", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_REPO_NAME"))
		assert.Equal(t, "development", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_TARGET"))
		assert.Equal(t, "succeeded", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_STATUS"))
		assert.Equal(t, "finished", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_RELEASE_EVENT"))
	})

	t.Run("ReturnsGitEventPropertiesAsEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		event := manifest.ZiplineeEvent{
			Fired: true,
			Git: &manifest.ZiplineeGitEvent{
				Event:      "push",
				Repository: "github.com/ziplineeci/ziplinee-ci-api",
				Branch:     "master",
			},
		}

		// act
		err := envvarHelper.setZiplineeEventEnvvars([]manifest.ZiplineeEvent{event})

		assert.Nil(t, err)
		envvars := envvarHelper.collectZiplineeEnvvars()
		assert.Equal(t, 3, len(envvars))
		assert.Equal(t, "push", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_GIT_EVENT"))
		assert.Equal(t, "github.com/ziplineeci/ziplinee-ci-api", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_GIT_REPOSITORY"))
		assert.Equal(t, "master", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_GIT_BRANCH"))
	})

	t.Run("ReturnsCronEventPropertiesAsEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		event := manifest.ZiplineeEvent{
			Fired: true,
			Cron: &manifest.ZiplineeCronEvent{
				Time: time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
			},
		}

		// act
		err := envvarHelper.setZiplineeEventEnvvars([]manifest.ZiplineeEvent{event})

		assert.Nil(t, err)
		envvars := envvarHelper.collectZiplineeEnvvars()
		assert.Equal(t, 1, len(envvars))
		assert.Equal(t, "2009-11-17T20:34:58.651387237Z", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_CRON_TIME"))
	})

	t.Run("ReturnsManualEventPropertiesAsEnvvars", func(t *testing.T) {

		_, _, envvarHelper, _ := getMocks()
		event := manifest.ZiplineeEvent{
			Fired: true,
			Manual: &manifest.ZiplineeManualEvent{
				UserID: "user@server.com",
			},
		}

		// act
		err := envvarHelper.setZiplineeEventEnvvars([]manifest.ZiplineeEvent{event})

		assert.Nil(t, err)
		envvars := envvarHelper.collectZiplineeEnvvars()
		assert.Equal(t, 1, len(envvars))
		assert.Equal(t, "user@server.com", envvarHelper.getZiplineeEnv("ZIPLINEE_TRIGGER_MANUAL_USER_ID"))
	})
}

func getMocks() (secretHelper crypt.SecretHelper, obfuscator Obfuscator, envvarHelper EnvvarHelper, whenEvaluator WhenEvaluator) {
	secretHelper = crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	obfuscator = NewObfuscator(secretHelper)
	envvarHelper = NewEnvvarHelper("TESTPREFIX_", secretHelper, obfuscator)
	whenEvaluator = NewWhenEvaluator(envvarHelper)

	envvarHelper.UnsetZiplineeEnvvars()

	return
}
