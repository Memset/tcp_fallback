tcp_fallback
============

This is a TCP proxy implementing a simple fallback mechanism.

All the requests will be forwarded to the first working backend on a list. When a backend
connection fails, the client connection is closed and the backend is marked as non-working
to be probed in background. Once the backend connectivity is restored, the backend is marked
back to be used by the proxy.

Because the proxy works at TPC level, it can be used with any TCP based service.

Build
-----

Go 1.0 or later is required: http://golang.org/

The proxy can be built with:

 go build tcp_fallback.go

Usage
-----

  tcp_fallback [flags] <local-address:port> [<remote-address:port>]+

flags:

  -debug=false: Enable verbose logging
  -probe-delay=30s: Interval to delay probes after backend error
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

