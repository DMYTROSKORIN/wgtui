// Package wgconfig parses and writes WireGuard interface configuration
// files (wg-quick style ini). It preserves comments, ordering and unknown
// fields, and provides a redactor that strips the [Interface] PrivateKey
// value from any text before it may leave the remote host or reach a log.
package wgconfig
