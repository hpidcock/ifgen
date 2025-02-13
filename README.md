# ifgen: an interface generator that makes writing testable go fun

Ifgen generates go interfaces based on what you use.

To use ifgen in your project, firstly add it to the `go.mod` tools section.
```
tool (
	github.com/hpidcock/ifgen
)
```

Then in packages that need generated interfaces for foreign types, add a
go generate comment similar to:

```go
//go:generate go tool ifgen -pkg mypackage github.com/google/go-github/v66/github:IssuesService=>GithubIssuesService
```

In this example, `mypackage` will have a package scoped type GithubIssuesService.
Ifgen creates two versions of the interface, one is called the "pure" interface,
the other is the impure interface.

The impure interface can be used by your code editor for autocompletion. While
the pure interface is generated to only have the methods required by your package.

To setup your code editor for use with ifgen, add the Go `impure` build tag.

.vscode/settings.json:
```json
{
    "go.buildTags": "impure"
}
```

.zed/settings.json
```json
{
  "lsp": {
    "gopls": {
      "initialization_options": {
        "buildFlags": ["-tags=impure"]
      }
    }
  }
}
```
