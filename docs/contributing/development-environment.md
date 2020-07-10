# Development Environment
A development container based on `ubuntu:bionic` has been provided,
so ensure docker is installed and the docker daemon is running.

* Open the project in [VSCode](https://code.visualstudio.com/).
* Install VSCode's [Remote Development Extension - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.vscode-remote-extensionpack).
* In the command palette (ctrl+shift+p on Windows/Linux,
command+shift+p on Mac), type "Reopen in Container".
* In the command palette type: "Go: Install/Update Tools" and select all.
* When all tools are finished installing, in the command palette type:
"Developer: Reload Window".
* The docker daemon is mapped from the host into the dev container,
so you can use docker and docker-compose commands from within the container
as if they were run on the host.