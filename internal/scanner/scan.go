/**

Scanner.

*/

package scanner

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sabhiram/go-gitignore"

	"github.com/92hackers/codetalks/internal"
	"github.com/92hackers/codetalks/internal/file"
	"github.com/92hackers/codetalks/internal/language"
	"github.com/92hackers/codetalks/internal/utils"
	"github.com/92hackers/codetalks/internal/view_mode"
)

var (
	uniqueDirSet   *utils.Set
	matchRegex     []*regexp.Regexp
	ignoreRegex    []*regexp.Regexp
	currentRootDir string
	vcsDirs        *utils.Set
	gitIgnoreMap   map[string]*ignore.GitIgnore // { currentRootDir: gitignore-patterns }
)

func init() {
	// Initialize the unique directory set
	uniqueDirSet = utils.NewSet()
	vcsDirs = utils.NewSet()
	{
		vcsDirs.Add(".git")
		vcsDirs.Add(".svn")
		vcsDirs.Add(".hg")
		vcsDirs.Add(".bzr")
		vcsDirs.Add(".cvs")
	}
	gitIgnoreMap = make(map[string]*ignore.GitIgnore)
}

func Config(
	matchRegexStr string,
	ignoreRegexStr string,
) {
	// Match regexs
	matchRegexStr = strings.TrimSpace(matchRegexStr)
	if len(matchRegexStr) > 0 {
		for _, regexStr := range strings.Split(matchRegexStr, " ") {
			matchRegex = append(matchRegex, regexp.MustCompile(regexStr))
		}
	}

	// Ignore regexs
	ignoreRegexStr = strings.TrimSpace(ignoreRegexStr)
	if len(ignoreRegexStr) > 0 {
		for _, regexStr := range strings.Split(ignoreRegexStr, " ") {
			ignoreRegex = append(ignoreRegex, regexp.MustCompile(regexStr))
		}
	}
}

func isVCSDir(path string) bool {
	return vcsDirs.Contains(path)
}

func isSpecifiedDepthDirs(path string, depth int) bool {
	segments := make([]string, 0, 10)
	for {
		dir, file := filepath.Split(path)
		trimedFile := strings.TrimSpace(file)
		if trimedFile != "" {
			segments = append(segments, trimedFile)
		}
		if dir == string(os.PathSeparator) || dir == "" {
			break
		}
		if strings.HasSuffix(dir, string(os.PathSeparator)) {
			dir = strings.TrimSuffix(dir, string(os.PathSeparator))
		}
		path = dir
	}
	return len(segments) == depth
}

func handler(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	leaf := filepath.Base(path)

	// Cut the root directory from the scanned path.
	cutRootDirPath := strings.TrimPrefix(path, currentRootDir)

	// dir
	if d.IsDir() {
		// Skip VCS directories
		if isVCSDir(leaf) {
			return fs.SkipDir
		}

		// Store the directory if viewMode is set to directory
		if internal.GlobalOpts.ViewMode == view_mode.ViewModeDirs {
			// Check Depth and store the directory
			// TODO: support multiple depth
			if isSpecifiedDepthDirs(cutRootDirPath, 1) {
				view_mode.SubDirs = append(view_mode.SubDirs, cutRootDirPath)
			}
		}

		// Skip directories that are ignored by gitignore
		//
		// Custom match regular expression has over precedence over gitignore patterns NOT works for directories.
		// For performance reasons, we skip directories that are ignored by gitignore.
		// To avoid scanning the files in the ignored directories.
		// To analyze thus directory, you can specific the directory as one of root directories.
		//
		if gi := gitIgnoreMap[currentRootDir]; gi != nil && gi.MatchesPath(cutRootDirPath) {
			return fs.SkipDir
		}

		return nil
	}

	// TODO: handle config file

	// Match regex filter
	{
		isMatched := false
		for _, re := range matchRegex {
			if re.MatchString(cutRootDirPath) {
				isMatched = true
				// Log
				if internal.GlobalOpts.IsDebugEnabled || internal.GlobalOpts.IsShowMatched {
					fmt.Println("File matched:", path)
				}
				break
			}
			if internal.GlobalOpts.IsDebugEnabled {
				fmt.Println("Not matched:", path, "with regexp:", re.String())
			}
		}
		if len(matchRegex) > 0 && !isMatched {
			return nil
		}

		// Matched by matchRegex
		// Custom match regular expression has over precedence over gitignore patterns

		// Check if the file is ignored by gitignore
		if !isMatched {
			gi := gitIgnoreMap[currentRootDir]
			if gi != nil && gi.MatchesPath(cutRootDirPath) {
				if internal.GlobalOpts.IsDebugEnabled {
					fmt.Println("File ignored by .gitignore rules:", path)
				}
				return nil
			}
		}
	}

	// Ignore regex filter
	for _, re := range ignoreRegex {
		if re.MatchString(cutRootDirPath) {
			if internal.GlobalOpts.IsDebugEnabled || internal.GlobalOpts.IsShowIgnored {
				fmt.Println("File ignored:", path, "with regexp:", re.String())
			}
			return nil
		}
	}

	// Skip unsupported file extensions
	fileExt := filepath.Ext(leaf)
	if internal.SupportedLanguages[fileExt] == nil {
		if internal.GlobalOpts.IsDebugEnabled {
			// fmt.Println("Unsupported file type:", path)
			utils.ErrorMsg("Unsupported file type: %s", path)
		}
		return nil
	}

	// Duplicate directory check
	if uniqueDirSet.Contains(path) {
		return nil
	}

	// debug
	if internal.GlobalOpts.IsDebugEnabled {
		fmt.Println("Add new file:", path)
	}

	// Create a new code file, skip if error
	codeFile, err := file.NewCodeFile(path)
	if err != nil {
		log.Println(err)
		return nil
	}

	// Add the code file to the language
	language.AddLanguage(fileExt, codeFile)

	// Add the directory to the unique directory set
	uniqueDirSet.Add(path)

	return nil
}

func addGitIgnorePatterns(rootDir string) {
	gi, _ := ignore.CompileIgnoreFile(filepath.Join(rootDir, ".gitignore"))
	gitIgnoreMap[rootDir] = gi
}

func Scan(rootDirs []string) {
	for _, dir := range rootDirs {
		currentRootDir = dir
		addGitIgnorePatterns(dir) // Only respect gitignore patterns for the root directory
		// Scan directory
		err := filepath.WalkDir(dir, handler)
		if err != nil {
			log.Fatal(err)
		}
	}
}
