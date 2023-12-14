#!/bin/bash
# Start distant daemon
distant manager listen --daemon
# Execute the main container command
exec "$@"

