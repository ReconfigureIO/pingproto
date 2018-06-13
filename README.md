# Pingproto

[![GoDoc](https://godoc.org/github.com/ReconfigureIO/pingproto?status.svg)](http://godoc.org/github.com/ReconfigureIO/pingproto)

Pingproto provides an application protocol which avoids being idle at the transport layer.

The use case is where you have a producer (e.g. some logs) which is very slow, you may not get bytes written for long periods of time. Proxies between the producer and consumer may kill the connection if it is idle for too long.

The solution is to put additional bytes on the wire. However, we don't want the application layer to see these bytes. So we have a protocol which goes `[length] [data] [length] [data]`. If we want to put bytes on the wire without them being seen by the application, we write a packet of length 0: `[length=0] [length] [data]`.

To use, on the writer side simply substitite your wire writer with `wApplication := pingproto.NewWriter(wire)`, and to read, use `rApplication := pingproto.NewReader(wire)`.

