#!/bin/bash
npm run build && cdk synth ;
find ./lib -name '*.js' -exec rm {} \;
 find ./lib -name '*.d.ts' -exec rm {} \;
