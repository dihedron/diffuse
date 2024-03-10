#!/bin/bash

#binaries := `find dist/ -name 'diffuse*'` 
for binary in `find dist/ -name 'diffuse'`; do 		
    echo $binary;
done;
