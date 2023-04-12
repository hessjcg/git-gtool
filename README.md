# Git CLI Tools for Googlers

*Description*

This project provides convenient tools to help Googlers work with their
github repos. 

## Features

### Automatically merge renovate all open PRs from renovate bot.

```
$ git gtool merge-renovate-prs
```

This will one-by-one merge any PRs opened by the Renovate Bot, until they are all
closed.

## Getting Started

1. Clone this repository
2. Run `go build -o 'git-gtool' main.go`
3. Put `git-gtool` into your path, for example: `cp git-gtool $HOME/bin/`

### Prerequisites
1. Install [Github CLI](https://cli.github.com/manual/installation)
2. Log in using the Github CLI

## Contributing

Contributions to this library are always welcome and highly encouraged.

See [CONTRIBUTING](CONTRIBUTING.md) for more information how to get started.

Please note that this project is released with a Contributor Code of Conduct. By participating in
this project you agree to abide by its terms. See [Code of Conduct](CODE_OF_CONDUCT.md) for more
information.

## License

Apache 2.0 - See [LICENSE](LICENSE) for more information.