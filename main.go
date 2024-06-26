package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"templ/configelements"
	"templ/repository"
	"templ/templatedirectories"
	"templ/templates"

	"golang.org/x/term"
)

func init() {
	createTemplDir()
}

func main() {
	list := flag.Bool("l", false, "list available templates and exit.")
	update := flag.Bool("u", false, "iterate over template repositories, calling git update.")
	url := flag.String("f", "", "clone/fetch a git repository from a url. Can be a github url or a local git repository.")
	variables := flag.Bool("v", false, "show only the variables from a file. If encountered, this will execute and exit.")

	usage := fmt.Sprintf("%s <templatename || templatename=variablesfile.yaml> <flags>\n\n"+
		"<templatename> can come in one of two forms. First is a template filename,"+
		"second is a template filename plus a yaml variables file of key: value pairs which"+
		"will populate the template file's variables. If no variables file is provided, the utility"+
		"operates like 'cat' on the file, printing it to stdout.\n\n"+
		"This utility can also be called in a pipeline as %s templatename | %s FOO=BAR BAM=BAS, for folks who"+
		"prefer not to have a variables file.\n\n"+
		"Templates can be stored in a git repository and downloaded with `templ -f https://path/to/git/repository.git`."+
		"git repository download support is provided by the awesome https://github.com/go-git/, and all protocols are supported."+
		"Only a `git pull` operation is supported - edit your templates with a text editor, commit to git, and"+
		" run `templ -u` to update your templates.\n\n"+
		"templ also supports project layouts. Supply a yaml file listing template names and template definition files.\n",
		filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))

	flag.Usage = func() { fmt.Println(usage); flag.PrintDefaults() }
	flag.Parse()

	fd := os.Stdin.Fd()

	//Someone's piping into the binary. Read from stdin and deal with the rendering.
	if !term.IsTerminal(int(fd)) {
		input, err := io.ReadAll(os.Stdin)

		if err != nil {
			_, file, line, _ := runtime.Caller(0)
			panic(fmt.Errorf("%s:%d: %v", file, line, err))
		}

		if *variables {
			templates.RetrieveVariables(string(input))
			os.Exit(0)
		}

		// There are 2 conditions that reach this line. The first is that we're piping input into templ.
		// The second is that we're running in a non-interactive shell, like ci/cd does.
		// In the first case, we want to deal with the input.
		// In the second case, we just want to fall out of this block and back to the default logic handling.
		if len(input) > 0 {

			variableDefinitions := flag.Args()
			hydratedTemplate, err := templates.RenderFromStdin(string(input), variableDefinitions)

			if err != nil {
				_, file, line, _ := runtime.Caller(0)
				panic(fmt.Errorf("%s:%d: %v", file, line, err))
			}

			fmt.Println(hydratedTemplate)

			os.Exit(0)
		}
	}

	if *list {
		files, err := templatedirectories.List()

		if err != nil {
			_, file, line, _ := runtime.Caller(0)
			panic(fmt.Errorf("%s:%d: %v", file, line, err))
		}

		for _, file := range files {
			fmt.Println(file)
		}

	}

	// If the user has provided a url, clone the repo
	if *url != "" {
		// I use github exclusively right now, so this is a safe bet. If I need to support more version control systems
		// I'll need to implement a case statement and switch on some piece of data to figure out the difference between
		// repo types. Probably need an extra cli flag.
		templRepo, err := repository.NewGitRepository(*url)

		if err != nil {
			panic(err)
		}

		err = templRepo.Fetch()

		if err != nil {
			panic(err)
		}
	}

	if *update {
		err := templatedirectories.Update()

		if err != nil {
			panic(err)
		}
	}

	if *variables {
		templateFilePaths, _, err := templates.FindTemplateAndVariableFiles(flag.Args())

		if err != nil {
			panic(err)
		}

		for _, file := range templateFilePaths {
			content, err := os.ReadFile(file)

			if err != nil {
				panic(err)
			}

			variables := templates.RetrieveVariables(string(content))

			if len(variables) == 0 {
				fmt.Printf("No variables detected in %s\n", file)
			} else {

				for _, variable := range variables {
					fmt.Println(variable)
				}
			}

		}
		os.Exit(0)
	}

	templateFilePaths, templateVariablesFilesPaths, err := templates.FindTemplateAndVariableFiles(flag.Args())

	if err != nil {
		_, file, line, _ := runtime.Caller(0)
		panic(fmt.Errorf("%s:%d: %v", file, line, err))
	}

	err = templates.RenderFromFiles(templateFilePaths, templateVariablesFilesPaths)

	if err != nil {
		_, file, line, _ := runtime.Caller(0)
		panic(fmt.Errorf("%s:%d: %v", file, line, err))
	}

}

//Helper functions

// createTemplDir requests information about the right path for templ's templates directory and creates that directory
// if need be. After calling this, our initial precondition should be met.
func createTemplDir() {
	templDir := configelements.NewTemplDir().TemplatesDir
	templDir, err := filepath.Abs(templDir)

	if err != nil {
		fmt.Printf("Tried to determine absolute path to the templ directory. Failed: %v", err)
		panic("")
	}

	_, err = os.Stat(templDir)

	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(templDir, 0700)

			if err != nil {
				fmt.Printf("Could not create configuration directory %s: %v", templDir, err)
			}
		}

		if err != nil {
			panic(err)
		}
	}
}
