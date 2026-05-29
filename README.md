# cmdfreq
- a CLI tool that analyzes you shell history and shows a ranked summary of your most used commands
## Requirements
- zsh - only zsh is supported wich history in either .histfile or .zsh_history - bash support will be added
- Extended History must be enabled, which includes timestamps and durations for each command. whithout it the history file won't be able to be parsed

## Setup
add the following to your ~/.zshrc
```bash
setopt EXTENDED_HISTORY
HISTFILE=~/.zsh_history # or ~/.histfile
HISTSIZE=10000
SAVEHIST=10000
```
then reload your config
```bash
source ~/.zshrc
```

## Installation
```bash
go install github.com/codestutis/cmdfreq@latest
```

## Usage
```bash
cmdfreq
```
Displays your top 20 most used commands, ranked by frequency
