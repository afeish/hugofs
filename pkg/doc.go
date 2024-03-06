// Package pkg holds all library-ish packages,
// which means this package is not an executable unit.
// Outside will compose different packages in the pkg to
// achieve different goals.
//
// The main purpose is to decouple packages and make
// each of them easy to test and maintain.
//
// This style is a kind like cargo member in one cargo workspace.
package pkg
