tcp_fallback
============

This is a TCP proxy implementing a simple fallback mechanism.

All the requests will be forwarded to the first working backend on a list. When a backend
connection fails, the client connection is closed and the backend is marked as non-working
to be probed in the background. Once the backend connectivity is restored, the backend is marked
back to be used by the proxy.

Because the proxy works at TCP level, it can be used with any TCP based service using one
single port.

Build
-----

Go 1.0 or later is required: http://golang.org/

Fetch and build with:

  go get github.com/memset/tcp_fallback

and this will build the binary in `$GOPATH/bin`. You can then modify the source and
submit patches.

Or checkout the source in the traditional way, change to the directory
and type:

  go build

Test the build with:

  go test

Usage
-----

  tcp_fallback [flags] <local-address:port> [<remote-address:port>]+

flags::

  -cpuprofile="": Write cpu profile to file if set
  -debug=false: Enable verbose logging
  -logfile="": Log into provided file
  -maxthreads=4: Maximum number of OS threads to use
  -probe-delay=30s: Interval to delay probes after backend error
  -quiet=false: Doesn't log anything
  -stats=15m0s: Interval to log stats
  -syslog=false: Use Syslog for logging
  -timeout=5s: Timeout for backend connection

Example:

 $ tcp_fallback -syslog 0.0.0.0:3306 192.168.0.10:3306 192.168.0.11:3306 192.168.0.12:3306

The proxy will listen for connections on all interfaces on port 3306 (MySQL) and will forward
the requests to 192.168.0.10:3306 (first backend), using 192.168.0.11:3306 and 192.168.0.12:3306
as fallback.

License
-------

This is free software under the terms of `MIT`_ license.

.. _`MIT`: http://en.wikipedia.org/wiki/MIT_License

Contact and support
-------------------

The project website is at:

https://github.com/memset/tcp_fallback

There you can file bug reports, ask for help or contribute patches.

Authors
-------

- Nick Craig-Wood <nick@memset.com>
- Juan J. Martinez <juan@memset.com>

