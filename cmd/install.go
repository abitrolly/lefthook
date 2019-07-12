package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var availableHooks = [...]string{
	"applypatch-msg",
	"pre-applypatch",
	"post-applypatch",
	"pre-commit",
	"prepare-commit-msg",
	"commit-msg",
	"post-commit",
	"pre-rebase",
	"post-checkout",
	"post-merge",
	"pre-push",
	"pre-receive",
	"update",
	"post-receive",
	"post-update",
	"pre-auto-gc",
	"post-rewrite",
}

var checkSumHook = "prepare-commit-msg"
var force bool

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Write basic configuration file in your project repository. Or initialize existed config",
	Run: func(cmd *cobra.Command, args []string) {
		InstallCmdExecutor(args, appFs)
	},
}

var appFs = afero.NewOsFs()

func init() {
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "reinstall hooks without checking config version")
	rootCmd.AddCommand(installCmd)
}

// InstallCmdExecutor execute basic configuration
func InstallCmdExecutor(args []string, fs afero.Fs) {
	if yes, _ := afero.Exists(fs, getConfigYamlPath()); yes {
		if !isConfigSync(fs) || force {
			log.Println(au.Cyan("SYNCING"), au.Bold("lefthook.yml"))
			DeleteGitHooks(fs)
			AddGitHooks(fs)
		}
	} else {
		AddConfigYaml(fs)
		addHook(checkSumHook, fs)
	}
}

// AddConfigYaml write lefthook.yml in root project directory
func AddConfigYaml(fs afero.Fs) {
	template := ""
	err := afero.WriteFile(fs, getConfigYamlPath(), []byte(template), defaultDirPermission)
	check(err)
	log.Println("Added config: ", getConfigYamlPath())
}

// AddGitHooks write existed directories in source_dir as hooks in .git/hooks
func AddGitHooks(fs afero.Fs) {
	// add directory hooks
	var dirsHooks []string
	dirEntities, err := afero.ReadDir(fs, getSourceDir())
	if err == nil {
		for _, f := range dirEntities {
			if f.IsDir() && contains(availableHooks[:], f.Name()) {
				dirsHooks = append(dirsHooks, f.Name())
			}
		}
	}

	var configHooks []string
	for _, key := range availableHooks {
		if viper.Get(key) != nil {
			configHooks = append(configHooks, key)
		}
	}

	unionHooks := append(dirsHooks, configHooks...)
	unionHooks = append(unionHooks, checkSumHook) // add special hook for Sync config
	unionHooks = uniqueStrSlice(unionHooks)
	log.Println(au.Cyan("SERVED HOOKS:"), au.Bold(strings.Join(unionHooks, ", ")))

	for _, key := range unionHooks {
		addHook(key, fs)
	}
}

func getConfigYamlPath() string {
	return filepath.Join(getRootPath(), configFileName) + configExtension
}

func getConfigLocalYamlPath() string {
	return filepath.Join(getRootPath(), configLocalFileName) + configExtension
}

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func isConfigSync(fs afero.Fs) bool {
	return configChecksum(fs) == recordedChecksum()
}

func configChecksum(fs afero.Fs) string {
	var returnMD5String string
	file, err := fs.Open(configFileName + configExtension)
	check(err)
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	check(err)

	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String = hex.EncodeToString(hashInBytes)

	return returnMD5String
}

func recordedChecksum() string {
	pattern := regexp.MustCompile(`(?:# lefthook_version: )(\w+)`)

	file, err := ioutil.ReadFile(filepath.Join(getGitHooksDir(), checkSumHook))
	if err != nil {
		return ""
	}

	match := pattern.FindStringSubmatch(string(file))

	return match[1]
}

func uniqueStrSlice(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
