Raft

This is a simple toy implementation of Raft using Go.  Its a distributed state machine that stores logs with a consensus of what the current
state is.

## Build

To build, just run:

```bash
cd path/to/raft
go build
```

This produces a single binary, `raft`.

Run it with `-h` to show built-in command line help.  It can either run as a server or be used to execute commands
against a running cluster.

## Running a cluster

To start a 3-node cluster, run the `raft` command three times, e.g. in three different terminals:

Terminal 1:
```
./raft -c ':9990,:9991,:9992' -p 9990
```

Terminal 2:
```
./raft -c ':9990,:9991,:9992' -p 9991
```

Terminal 3:
```
./raft -c ':9990,:9991,:9992' -p 9992
```

You can of course always choose to run them as daemon processes that write to files.

All three nodes should come up and elect a leader.  They'll store their log files in your home directory under `~/.goraft/<agentId>` where `agentId` is 
combination `hostname_port`.

## Client commands

A log is really a command from the client to the cluster.  The server can choose to do whatever it wants with the log.

For example, if you were building a database it could change the internal state somehow.  This implementation doesn't do anything
fancy with its logs though.

### Appending a log

All commands are appended to the log.

The following would append the log "I like beer." to cluster member ":9990".  It doesn't matter if its the leader or not, it will
forward its request on (and tell you so much).  

```
./raft -C "clientcommand" -m "I like beer." -S ":9990"
```

This doesn't make the server do much.

### Print and Count

A message of `print` or `count` cause the server to write to their standard out when processed.

```
./raft -C "clientcommand" -m "count" -S ":9990"
./raft -C "clientcommand" -m "print" -S ":9990"
```

Would output the following on each server (depending on the state of the logs):

```
2024/02/23 17:52:24 Logs [i like beer count print]
2024/02/23 18:42:30 Count 3
```

## Changing the cluster

It wouldn't be very fault tolerant machine if you couldn't add and remove machines from the cluster.

To add a new machine, start it with its agent id out of the configuratioion

```
./raft -c ':9990,:9991,:9992' -p ':9993'
```

Notice how `:9993` is not in the provided list.

Then issue an `addserver` command to set the configuration to include the new server:

```
./raft -C addserver -m ":9993" -S ":9990"
```

Each server will announce the new peer, e.g.:

```
2024/02/23 18:43:20 Added new peer :9993
```

And the peer will be updated with logs it doesn't have.  Any logs that it adds are processed.

The machine can similarly be removed with `removeserver`

```
./raft -C removeserver -m ":9993" -S ":9990"
```
