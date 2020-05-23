# Development Environment
A development container based on `ubuntu:bionic` has been provided,
so ensure docker is installed and the docker daemon is running.

If using VSCode's [Remote Development Extension - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.vscode-remote-extensionpack):
* Open the project in VSCode.
* In the command palette (ctrl+shift+p on Windows/Linux,
command+shift+p on Mac), type "Reopen in Container".
* In the command palette type: "Go: Install/Update Tools" and select all.
* When all tools are finished installing, in the command palette type:
"Developer: Reload Window".
* The docker daemon is mapped from the host into the dev container,
so you can use docker and docker-compose commands from within the container
as if they were run on the host.

If using vim:
* The development container includes the
[basic version of vim-awesome](https://github.com/amix/vimrc#how-to-install-the-basic-version),
[vim-go](https://github.com/fatih/vim-go), and [NERDTree](https://github.com/preservim/nerdtree).
* Build the development container:
`docker build -f .devcontainer/Dockerfile -t dev .`
* Mount the root directory into the container, and drop into a bash shell:
`docker run -it -v ${PWD}:/workspaces/docker-lock -v /var/run/docker.sock:/var/run/docker.sock dev`
* Open vim and type `:GoInstallBinaries` to initialize `vim-go`
* When all the tools have been installed, close and reopen vim.