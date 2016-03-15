% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014

# NAME

docker-rm - Remove one or more containers.

# SYNOPSIS

**docker rm** [**-f**|**--force**[=*false*] [**-l**|**--link**[=*false*] [**-v**|
**--volumes**[=*false*]
CONTAINER [CONTAINER...]

# DESCRIPTION

**docker rm** will remove one or more containers from the host node. The
container name or ID can be used. This does not remove images. You cannot
remove a running container unless you use the \fB-f\fR option. To see all
containers on a host use the **docker ps -a** command.

# OPTIONS

**-f**, **--force**=*true*|*false*
   When set to true, force the removal of the container. The default is
*false*.

**-l**, **--link**=*true*|*false*
   When set to true, remove the specified link and not the underlying
container. The default is *false*.

**-v**, **--volumes**=*true*|*false*
   When set to true, remove the volumes associated to the container. The
default is *false*.

# EXAMPLES

##Removing a container using its ID##

To remove a container using its ID, find either from a **docker ps -a**
command, or use the ID returned from the **docker run** command, or retrieve
it from a file used to store it using the **docker run --cidfile**:

    docker rm abebf7571666

##Removing a container using the container name##

The name of the container can be found using the **docker ps -a**
command. The use that name as follows:

    docker rm hopeful_morse

# HISTORY

April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
