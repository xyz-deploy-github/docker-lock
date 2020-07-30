# Command Line Flags

`docker-lock` supports a variety of command line flags to customize behavior.

For instance, by default, `docker-lock` looks for files named `Dockerfile`,
`docker-compose.yaml`, and `docker-compose.yml` in the directory from which
the command is run. However, you may want `docker-lock` to find all
`Dockerfile`s in your project.

To do so, you could specify the command line flag, `--dockerfile-recursive`,
to the `generate` command as in:

```
docker lock generate --dockerfile-recursive
```

To see available command line flags, run commands with `--help`. For instance:

```
docker lock --help
docker lock generate --help
docker lock verify --help
docker lock rewrite --help
docker lock version --help
```

# Configuration File
Instead of specifying command line flags, you can specify options in a
configuration file, `.docker-lock.yml`, in the directory from which the
command will be run.

All keys have the same names as the command line flags.

For instance, in the recursive example, the file, `.docker-lock.yml`,
would look like:

```
dockerfile-recursive: "true"
```