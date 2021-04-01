@echo off

protoc --gofast_out=../message --plugin "pub run protoc_plugin" ./message.proto