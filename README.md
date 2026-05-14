## Core idea

This project was inspired by [Tsoding](https://github.com/tsoding)'s project in C,
called [nob.h](https://github.com/tsoding/nob.h). Main philosophy is - you should be
able to build X language project using only X language. In this specific case, build
Go project using Go language

## Examples

### Simple example

nob.go:
```go
package main

import (
	"os"

	nob "github.com/Fipaan/nob.go"
)

func main() {
	nob.GoRebuildUrself("go", "build", "-o", "nob")

	cmd := nob.CmdInit()

	cmd.Push("go", "run", "./cmd")
    if !cmd.Run() { os.Exit(1) }
}
```

### More complex example

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	nob "github.com/Fipaan/nob.go"
)

type Service struct {
	Name    string
	DB_User string
	DB_Host string
	DB_Port uint
	DB_Name string
}

func (s Service) Path() string {
	return filepath.Join(nob.ProgramDir(), fmt.Sprintf("%v-service", s.Name))
}

var SERVICES = []Service{
	{Name: "user",   DB_User: "postgres", DB_Host: nob.TryEnv("DB_HOST", "localhost")},
	{Name: "admin",  DB_User: "postgres", DB_Host: nob.TryEnv("DB_HOST", "localhost")},
	{Name: "fandom", DB_User: "postgres", DB_Host: nob.TryEnv("DB_HOST", "localhost")},
}

func listServices() {
	fmt.Printf("Services:\n")
	for i := 0; i < len(SERVICES); i++ {
		service := SERVICES[i]
		fmt.Printf("  %v\n", service.Name)
	}
}

func syncConfig(cmd *nob.Cmd) bool {
	src := filepath.Join(nob.ProgramDir(), "config", "config.go")
	nobSrc := nob.SourceName(0)

	for _, s := range SERVICES {
		configFolder := filepath.Join(s.Path(), "internal", "config")
		err := nob.MkdirIfNotExists(configFolder)
		if err != nil {
			fmt.Printf("ERROR: couldn't create %q: %v\n", configFolder, err)
			return false
		}

		dst := filepath.Join(configFolder, "common.go")
		// rebuild config depends on either nob.go or config.go update
		needs, err := nob.NeedsRebuild(dst, src, nobSrc)
		if err != nil {
			fmt.Printf("ERROR: syncConfig: %v\n", err)
			return false
		}
		if !needs {
			fmt.Printf("INFO: config is up to date: %q\n", s.Name)
			continue
		}

		err = nob.CopyFile(src, dst)
		if err != nil {
			fmt.Printf("ERROR: couldn't copy config to %q: %v\n", s.Name, err)
			return false
		}
		fmt.Printf("INFO: config updated for %q\n", s.Name)
	}
	return true
}

func startService(cmd *nob.Cmd, name string) bool {
	for _, s := range SERVICES {
		if s.Name != name { continue }

		fmt.Printf("CMD: Starting service %q\n", s.Name)
		saved := cmd.WalkIn(s.Path())

		cmd.Push("go", "mod", "tidy")
		if !cmd.Run() { return false }

		cmd.Push("go", "run", "./cmd")
		if !cmd.Run() { return false }

		cmd.Dir = saved
		return true
	}
	fmt.Printf("ERROR: unknown service %q\n", name)
	return false
}

func startFrontend(cmd *nob.Cmd) bool {
	saved := cmd.WalkIn(filepath.Join(nob.ProgramDir(), "frontend"))
	cmd.Push("npm", "run", "start")
	if !cmd.Run() { return false }
	cmd.Dir = saved
	return true
}

func main() {
	nob.GoRebuildUrself("go", "build", "-o", "nob")

	var dotenv   bool
	var service  string
	var frontend bool
	var isList   bool
	flag.BoolVar  (&dotenv,   "dotenv",   false, "reads from .env")
	flag.BoolVar  (&isList,   "l",        false, "list services")
	flag.StringVar(&service,  "s",        "",    "start service")
	flag.BoolVar  (&frontend, "frontend", false, "start frontend")
	flag.Parse()

	cmd := nob.CmdInit()
	cmd.Dotenv = dotenv

	if isList {
		listServices()
		return
	}
	if frontend {
		ok := startFrontend(cmd)
		if !ok { os.Exit(1) }
		return
	}
	if service != "" {
		if !syncConfig(cmd) 		   { os.Exit(1) }
		if !startService(cmd, service) { os.Exit(1) }
		return
	}

	fmt.Printf("ERROR: no command specified\n")
	flag.Usage()
	os.Exit(1)
}
```
