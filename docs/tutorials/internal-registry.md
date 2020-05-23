# Internal Registries
`docker-lock` supports a variety of container registries. Although registries
often differ slightly, they typically implement the
[Docker Registry HTTP API V2 Specification](https://docs.docker.com/registry/spec/api/)
and the associated
[Token Authentication Specification](https://docs.docker.com/registry/spec/auth/token/).

With these specifications in mind, `docker-lock` ships with an
implementation for internal registries whose behavior can be controlled
through environment variables.

> Note: If `docker-lock`'s implementation for internal registries does not
suffice for your use case, you can easily extend `docker-lock` to support
any registry by [implementing a simple interface](../../registry/wrapper.go).

# Example
In this example, we will:

* Run and populate an [internal registry](https://docs.docker.com/registry/deploying/) with images
* Generate a Lockfile with `docker-lock`

## Setup Internal Registry
To run the registry:
```
docker run -d -p 5000:5000 --restart=always --name registry registry:2
```
To populate the registry, pull a public image:
```
docker pull ubuntu:16.04
```
Tag the image:
```
docker tag ubuntu:16.04 localhost:5000/my-ubuntu
```
Finally, push the image to the registry:
```
docker push localhost:5000/my-ubuntu
```
## Generate Lockfile
Create a Dockerfile that contains:
```
FROM localhost:5000/my-ubuntu:latest
```
Create a .env file that contains:
```
INTERNAL_REGISTRY_URL=http://localhost:5000
INTERNAL_PREFIX=localhost:5000/
INTERNAL_STRIP_PREFIX=true
```
> Note: If you do not want to use a .env file, just make sure to export
the same environment variables.

Generate a Lockfile:
```
docker lock generate
```

Inside `docker-lock.json` you will now find your image's digest.

Let's understand the environment variables:

`INTERNAL_REGISTRY_URL` defines the location of the internal registry.

`INTERNAL_PREFIX` lets `docker-lock` know which registry to query when an
image is found in a Dockerfile. In this case, the Dockerfile's image
has a prefix of `localhost:5000/`, so the `INTERNAL_PREFIX` should be the same.

While our internal prefix refers to the local registry URL, this is unsightly
and restrictive. Instead, it is common to prefix images with a namespace
such as your organization.

To see this, modify the `INTERNAL_PREFIX` so that the .env file resembles:
```
INTERNAL_REGISTRY_URL=http://localhost:5000
INTERNAL_PREFIX=my-org/
INTERNAL_STRIP_PREFIX=true
```
Change the Dockerfile:
```
FROM my-org/my-ubuntu:latest
```
Generate a Lockfile:
```
docker lock generate
```
Inside `docker-lock.json`, you will now see that `localhost:5000/` has been
replaced with `my-org/`.

`INTERNAL_STRIP_PREFIX` tells `docker-lock` if the prefix should be considered
part of the repository name.
The line
```
FROM localhost:5000/my-ubuntu:latest
```
can be more generically thought of as
```
FROM prefix/repository:reference
```
In some registries, a prefix is non-existent/is considered part of
the repository name. In these cases, `INTERNAL_STRIP_PREFIX` should be `false`.

Finally, there is one environment variable we have not defined,
`INTERNAL_TOKEN_URL`. If it is defined, `docker-lock` will query
that URL for a bearer token, expecting a json response with the key `token`.
This token will be used when querying for the digest.

Typically, requests for the authorization token happens per repository. If
this is the case, the variable could be:
```
INTERNAL_TOKEN_URL="https://localhost:5000/v2/auth?scope=repository:<REPO>:pull"
```
`docker-lock` will substitute `<REPO>` with the repository name, obeying the
variable `INTERNAL_STRIP_PREFIX`.
