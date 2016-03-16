page_title: Registry Documentation
page_description: Documentation for docker Registry and Registry API
page_keywords: docker, registry, api, index

# Registry & Index Spec

## The 3 roles

### Index

The Index is responsible for centralizing information about:

 - User accounts
 - Checksums of the images
 - Public namespaces

The Index has different components:

 - Web UI
 - Meta-data store (comments, stars, list public repositories)
 - Authentication service
 - Tokenization

The index is authoritative for those information.

We expect that there will be only one instance of the index, run and
managed by Docker Inc.

### Registry

 - It stores the images and the graph for a set of repositories
 - It does not have user accounts data
 - It has no notion of user accounts or authorization
 - It delegates authentication and authorization to the Index Auth
   service using tokens
 - It supports different storage backends (S3, cloud files, local FS)
 - It doesn't have a local database
 - [Source Code](https://github.com/dotcloud/docker-registry)

We expect that there will be multiple registries out there. To help to
grasp the context, here are some examples of registries:

 - **sponsor registry**: such a registry is provided by a third-party
   hosting infrastructure as a convenience for their customers and the
   docker community as a whole. Its costs are supported by the third
   party, but the management and operation of the registry are
   supported by dotCloud. It features read/write access, and delegates
   authentication and authorization to the Index.
 - **mirror registry**: such a registry is provided by a third-party
   hosting infrastructure but is targeted at their customers only. Some
   mechanism (unspecified to date) ensures that public images are
   pulled from a sponsor registry to the mirror registry, to make sure
   that the customers of the third-party provider can “docker pull”
   those images locally.
 - **vendor registry**: such a registry is provided by a software
   vendor, who wants to distribute docker images. It would be operated
   and managed by the vendor. Only users authorized by the vendor would
   be able to get write access. Some images would be public (accessible
   for anyone), others private (accessible only for authorized users).
   Authentication and authorization would be delegated to the Index.
   The goal of vendor registries is to let someone do “docker pull
   basho/riak1.3” and automatically push from the vendor registry
   (instead of a sponsor registry); i.e. get all the convenience of a
   sponsor registry, while retaining control on the asset distribution.
 - **private registry**: such a registry is located behind a firewall,
   or protected by an additional security layer (HTTP authorization,
   SSL client-side certificates, IP address authorization...). The
   registry is operated by a private entity, outside of dotCloud's
   control. It can optionally delegate additional authorization to the
   Index, but it is not mandatory.

> **Note:** The latter implies that while HTTP is the protocol
> of choice for a registry, multiple schemes are possible (and
> in some cases, trivial):
>
> - HTTP with GET (and PUT for read-write registries);
> - local mount point;
> - remote docker addressed through SSH.

The latter would only require two new commands in docker, e.g.
`registryget` and `registryput`,
wrapping access to the local filesystem (and optionally doing
consistency checks). Authentication and authorization are then delegated
to SSH (e.g. with public keys).

### Docker

On top of being a runtime for LXC, Docker is the Registry client. It
supports:

 - Push / Pull on the registry
 - Client authentication on the Index

## Workflow

### Pull

![](/static_files/docker_pull_chart.png)

1.  Contact the Index to know where I should download “samalba/busybox”
2.  Index replies: a. `samalba/busybox` is on Registry A b. here are the
    checksums for `samalba/busybox` (for all layers) c. token
3.  Contact Registry A to receive the layers for `samalba/busybox` (all of
    them to the base image). Registry A is authoritative for “samalba/busybox”
    but keeps a copy of all inherited layers and serve them all from the same
    location.
4.  registry contacts index to verify if token/user is allowed to download images
5.  Index returns true/false lettings registry know if it should proceed or error
    out
6.  Get the payload for all layers

It's possible to run:

    $ docker pull https://<registry>/repositories/samalba/busybox

In this case, Docker bypasses the Index. However the security is not
guaranteed (in case Registry A is corrupted) because there won't be any
checksum checks.

Currently registry redirects to s3 urls for downloads, going forward all
downloads need to be streamed through the registry. The Registry will
then abstract the calls to S3 by a top-level class which implements
sub-classes for S3 and local storage.

Token is only returned when the `X-Docker-Token`
header is sent with request.

Basic Auth is required to pull private repos. Basic auth isn't required
for pulling public repos, but if one is provided, it needs to be valid
and for an active account.

**API (pulling repository foo/bar):**

1.  (Docker -> Index) GET /v1/repositories/foo/bar/images:

        **Headers**:
        Authorization: Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==
        X-Docker-Token: true
        
        **Action**:
        (looking up the foo/bar in db and gets images and checksums
        for that repo (all if no tag is specified, if tag, only
        checksums for those tags) see part 4.4.1)

2.  (Index -> Docker) HTTP 200 OK

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=write
        X-Docker-Endpoints: registry.docker.io [,registry2.docker.io]
        
        **Body**:
        Jsonified checksums (see part 4.4.1)

3.  (Docker -> Registry) GET /v1/repositories/foo/bar/tags/latest

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=write

4.  (Registry -> Index) GET /v1/repositories/foo/bar/images

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=read
        
        **Body**:
        <ids and checksums in payload>
        
        **Action**:
        (Lookup token see if they have access to pull.)
        
        If good:
        HTTP 200 OK Index will invalidate the token
        
        If bad:
        HTTP 401 Unauthorized

5.  (Docker -> Registry) GET /v1/images/928374982374/ancestry

        **Action**:
        (for each image id returned in the registry, fetch /json + /layer)

> **Note**:
> If someone makes a second request, then we will always give a new token,
> never reuse tokens.

### Push

![](/static_files/docker_push_chart.png)

1.  Contact the index to allocate the repository name “samalba/busybox”
    (authentication required with user credentials)
2.  If authentication works and namespace available, “samalba/busybox”
    is allocated and a temporary token is returned (namespace is marked
    as initialized in index)
3.  Push the image on the registry (along with the token)
4.  Registry A contacts the Index to verify the token (token must
    corresponds to the repository name)
5.  Index validates the token. Registry A starts reading the stream
    pushed by docker and store the repository (with its images)
6.  docker contacts the index to give checksums for upload images

> **Note:**
> **It's possible not to use the Index at all!** In this case, a deployed
> version of the Registry is deployed to store and serve images. Those
> images are not authenticated and the security is not guaranteed.

> **Note:**
> **Index can be replaced!** For a private Registry deployed, a custom
> Index can be used to serve and validate token according to different
> policies.

Docker computes the checksums and submit them to the Index at the end of
the push. When a repository name does not have checksums on the Index,
it means that the push is in progress (since checksums are submitted at
the end).

**API (pushing repos foo/bar):**

1.  (Docker -> Index) PUT /v1/repositories/foo/bar/

        **Headers**:
        Authorization: Basic sdkjfskdjfhsdkjfh== X-Docker-Token:
        true

        **Action**:
        - in index, we allocated a new repository, and set to
        initialized

        **Body**:
        (The body contains the list of images that are going to be
        pushed, with empty checksums. The checksums will be set at
        the end of the push):

        [{“id”: “9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f”}]

2.  (Index -> Docker) 200 Created

        **Headers**:
        - WWW-Authenticate: Token
        signature=123abc,repository=”foo/bar”,access=write
        - X-Docker-Endpoints: registry.docker.io [,
        registry2.docker.io]

3.  (Docker -> Registry) PUT /v1/images/98765432_parent/json

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=write

4.  (Registry->Index) GET /v1/repositories/foo/bar/images

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=write

        **Action**:
        - Index:
        will invalidate the token.
        - Registry:
        grants a session (if token is approved) and fetches
        the images id

5.  (Docker -> Registry) PUT /v1/images/98765432_parent/json

        **Headers**::
        - Authorization: Token
        signature=123abc,repository=”foo/bar”,access=write
        - Cookie: (Cookie provided by the Registry)

6.  (Docker -> Registry) PUT /v1/images/98765432/json

        **Headers**:
        - Cookie: (Cookie provided by the Registry)

7.  (Docker -> Registry) PUT /v1/images/98765432_parent/layer

        **Headers**:
        - Cookie: (Cookie provided by the Registry)

8.  (Docker -> Registry) PUT /v1/images/98765432/layer

        **Headers**:
        X-Docker-Checksum: sha256:436745873465fdjkhdfjkgh

9.  (Docker -> Registry) PUT /v1/repositories/foo/bar/tags/latest

        **Headers**:
        - Cookie: (Cookie provided by the Registry)

        **Body**:
        “98765432”

10. (Docker -> Index) PUT /v1/repositories/foo/bar/images

        **Headers**:
        Authorization: Basic 123oislifjsldfj== X-Docker-Endpoints:
        registry1.docker.io (no validation on this right now)

        **Body**:
        (The image, id`s, tags and checksums)
        [{“id”:
        “9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f”,
        “checksum”:
        “b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087”}]

        **Return**: HTTP 204

> **Note:** If push fails and they need to start again, what happens in the index,
> there will already be a record for the namespace/name, but it will be
> initialized. Should we allow it, or mark as name already used? One edge
> case could be if someone pushes the same thing at the same time with two
> different shells.

If it's a retry on the Registry, Docker has a cookie (provided by the
registry after token validation). So the Index won't have to provide a
new token.

### Delete

If you need to delete something from the index or registry, we need a
nice clean way to do that. Here is the workflow.

1.  Docker contacts the index to request a delete of a repository
    `samalba/busybox` (authentication required with user credentials)
2.  If authentication works and repository is valid, `samalba/busybox`
    is marked as deleted and a temporary token is returned
3.  Send a delete request to the registry for the repository (along with
    the token)
4.  Registry A contacts the Index to verify the token (token must
    corresponds to the repository name)
5.  Index validates the token. Registry A deletes the repository and
    everything associated to it.
6.  docker contacts the index to let it know it was removed from the
    registry, the index removes all records from the database.

> **Note**:
> The Docker client should present an "Are you sure?" prompt to confirm
> the deletion before starting the process. Once it starts it can't be
> undone.

**API (deleting repository foo/bar):**

1.  (Docker -> Index) DELETE /v1/repositories/foo/bar/

        **Headers**:
        Authorization: Basic sdkjfskdjfhsdkjfh== X-Docker-Token:
        true

        **Action**:
        - in index, we make sure it is a valid repository, and set
        to deleted (logically)

        **Body**:
        Empty

2.  (Index -> Docker) 202 Accepted

        **Headers**:
        - WWW-Authenticate: Token
        signature=123abc,repository=”foo/bar”,access=delete
        - X-Docker-Endpoints: registry.docker.io [,
        registry2.docker.io]
        # list of endpoints where this repo lives.

3.  (Docker -> Registry) DELETE /v1/repositories/foo/bar/

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=delete

4.  (Registry->Index) PUT /v1/repositories/foo/bar/auth

        **Headers**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=delete

        **Action**:
        - Index:
        will invalidate the token.
        - Registry:
        deletes the repository (if token is approved)

5.  (Registry -> Docker) 200 OK

        200 If success 403 if forbidden 400 if bad request 404
        if repository isn't found

6.  (Docker -> Index) DELETE /v1/repositories/foo/bar/

        **Headers**:
        Authorization: Basic 123oislifjsldfj== X-Docker-Endpoints:
        registry-1.docker.io (no validation on this right now)
        
        **Body**:
        Empty
        
        **Return**: HTTP 200

## How to use the Registry in standalone mode

The Index has two main purposes (along with its fancy social features):

 - Resolve short names (to avoid passing absolute URLs all the time):

    username/projectname ->
    https://registry.docker.io/users/<username>/repositories/<projectname>/
    team/projectname ->
    https://registry.docker.io/team/<team>/repositories/<projectname>/

 - Authenticate a user as a repos owner (for a central referenced
    repository)

### Without an Index

Using the Registry without the Index can be useful to store the images
on a private network without having to rely on an external entity
controlled by Docker Inc.

In this case, the registry will be launched in a special mode
(–standalone? –no-index?). In this mode, the only thing which changes is
that Registry will never contact the Index to verify a token. It will be
the Registry owner responsibility to authenticate the user who pushes
(or even pulls) an image using any mechanism (HTTP auth, IP based,
etc...).

In this scenario, the Registry is responsible for the security in case
of data corruption since the checksums are not delivered by a trusted
entity.

As hinted previously, a standalone registry can also be implemented by
any HTTP server handling GET/PUT requests (or even only GET requests if
no write access is necessary).

### With an Index

The Index data needed by the Registry are simple:

 - Serve the checksums
 - Provide and authorize a Token

In the scenario of a Registry running on a private network with the need
of centralizing and authorizing, it's easy to use a custom Index.

The only challenge will be to tell Docker to contact (and trust) this
custom Index. Docker will be configurable at some point to use a
specific Index, it'll be the private entity responsibility (basically
the organization who uses Docker in a private environment) to maintain
the Index and the Docker's configuration among its consumers.

## The API

The first version of the api is available here:
[https://github.com/jpetazzo/docker/blob/acd51ecea8f5d3c02b00a08176171c59442df8b3/docs/images-repositories-push-pull.md](https://github.com/jpetazzo/docker/blob/acd51ecea8f5d3c02b00a08176171c59442df8b3/docs/images-repositories-push-pull.md)

### Images

The format returned in the images is not defined here (for layer and
JSON), basically because Registry stores exactly the same kind of
information as Docker uses to manage them.

The format of ancestry is a line-separated list of image ids, in age
order, i.e. the image's parent is on the last line, the parent of the
parent on the next-to-last line, etc.; if the image has no parent, the
file is empty.

    GET /v1/images/<image_id>/layer
    PUT /v1/images/<image_id>/layer
    GET /v1/images/<image_id>/json
    PUT /v1/images/<image_id>/json
    GET /v1/images/<image_id>/ancestry
    PUT /v1/images/<image_id>/ancestry

### Users

### Create a user (Index)

    POST /v1/users:

    **Body**:
    {"email": "[sam@dotcloud.com](mailto:sam%40dotcloud.com)",
    "password": "toto42", "username": "foobar"`}

    **Validation**:
    - **username**: min 4 character, max 30 characters, must match the
    regular expression [a-z0-9_].
    - **password**: min 5 characters

    **Valid**: return HTTP 200

Errors: HTTP 400 (we should create error codes for possible errors) -
invalid json - missing field - wrong format (username, password, email,
etc) - forbidden name - name already exists

> **Note**:
> A user account will be valid only if the email has been validated (a
> validation link is sent to the email address).

### Update a user (Index)

    PUT /v1/users/<username>

    **Body**:
    {"password": "toto"}

> **Note**:
> We can also update email address, if they do, they will need to reverify
> their new email address.

### Login (Index)

Does nothing else but asking for a user authentication. Can be used to
validate credentials. HTTP Basic Auth for now, maybe change in future.

GET /v1/users

    **Return**:
     - Valid: HTTP 200
     - Invalid login: HTTP 401
     - Account inactive: HTTP 403 Account is not Active

### Tags (Registry)

The Registry does not know anything about users. Even though
repositories are under usernames, it's just a namespace for the
registry. Allowing us to implement organizations or different namespaces
per user later, without modifying the Registry'sAPI.

The following naming restrictions apply:

 - Namespaces must match the same regular expression as usernames (See
    4.2.1.)
 - Repository names must match the regular expression [a-zA-Z0-9-_.]

### Get all tags:

GET /v1/repositories/<namespace>/<repository_name>/tags

    **Return**: HTTP 200
    { "latest":
    "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
    “0.1.1”:
    “b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087” }

    **4.3.2 Read the content of a tag (resolve the image id):**

    GET /v1/repositories/<namespace>/<repo_name>/tags/<tag>

    **Return**:
    "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f"

    **4.3.3 Delete a tag (registry):**

    DELETE /v1/repositories/<namespace>/<repo_name>/tags/<tag>

### 4.4 Images (Index)

For the Index to “resolve” the repository name to a Registry location,
it uses the X-Docker-Endpoints header. In other terms, this requests
always add a `X-Docker-Endpoints` to indicate the
location of the registry which hosts this repository.

**4.4.1 Get the images:**

    GET /v1/repositories/<namespace>/<repo_name>/images

    **Return**: HTTP 200
    [{“id”:
    “9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f”,
    “checksum”:
    “[md5:b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087](md5:b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087)”}]

### Add/update the images:

You always add images, you never remove them.

    PUT /v1/repositories/<namespace>/<repo_name>/images

    **Body**:
    [ {“id”:
    “9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f”,
    “checksum”:
    “sha256:b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087”}
    ]

    **Return**: 204

### Repositories

### Remove a Repository (Registry)

DELETE /v1/repositories/<namespace>/<repo_name>

Return 200 OK

### Remove a Repository (Index)

This starts the delete process. see 2.3 for more details.

DELETE /v1/repositories/<namespace>/<repo_name>

Return 202 OK

## Chaining Registries

It's possible to chain Registries server for several reasons:

 - Load balancing
 - Delegate the next request to another server

When a Registry is a reference for a repository, it should host the
entire images chain in order to avoid breaking the chain during the
download.

The Index and Registry use this mechanism to redirect on one or the
other.

Example with an image download:

On every request, a special header can be returned:

    X-Docker-Endpoints: server1,server2

On the next request, the client will always pick a server from this
list.

## Authentication & Authorization

### On the Index

The Index supports both “Basic” and “Token” challenges. Usually when
there is a `401 Unauthorized`, the Index replies
this:

    401 Unauthorized
    WWW-Authenticate: Basic realm="auth required",Token

You have 3 options:

1.  Provide user credentials and ask for a token

        **Header**:
        - Authorization: Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==
        - X-Docker-Token: true
        
        In this case, along with the 200 response, you'll get a new token
        (if user auth is ok): If authorization isn't correct you get a 401
        response. If account isn't active you will get a 403 response.
        
        **Response**:
        - 200 OK
        - X-Docker-Token: Token
          signature=123abc,repository=”foo/bar”,access=read
    

2.  Provide user credentials only

        **Header**:
        Authorization: Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==

3.  Provide Token

        **Header**:
        Authorization: Token
        signature=123abc,repository=”foo/bar”,access=read

### 6.2 On the Registry

The Registry only supports the Token challenge:

    401 Unauthorized
    WWW-Authenticate: Token

The only way is to provide a token on `401 Unauthorized`
responses:

    Authorization: Token signature=123abc,repository="foo/bar",access=read

Usually, the Registry provides a Cookie when a Token verification
succeeded. Every time the Registry passes a Cookie, you have to pass it
back the same cookie.:

    200 OK
    Set-Cookie: session="wD/J7LqL5ctqw8haL10vgfhrb2Q=?foo=UydiYXInCnAxCi4=&timestamp=RjEzNjYzMTQ5NDcuNDc0NjQzCi4="; Path=/; HttpOnly

Next request:

    GET /(...)
    Cookie: session="wD/J7LqL5ctqw8haL10vgfhrb2Q=?foo=UydiYXInCnAxCi4=&timestamp=RjEzNjYzMTQ5NDcuNDc0NjQzCi4="

## Document Version

 - 1.0 : May 6th 2013 : initial release
 - 1.1 : June 1st 2013 : Added Delete Repository and way to handle new
    source namespace.

