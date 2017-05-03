#!/usr/bin/env node
'use strict';

const process = require('process');
const webpack = require('webpack');
const WebPackConfig = require(process.env.WEBPACK_CONFIG)();

webpack(WebPackConfig).run((err, stats) => {
  if (err) {
    throw err;
  }

  const result = stats.toJson();
  if (result.errors.length > 0) {
    log(`Compilation failed with ${result.errors.length} errors\n`);

    result.errors.forEach((error) => {
      log(error.split('at Parser.pp.raise')[0]); // Remove the part of the babel stacks that are not useful
    });
    throw result;
  }
});
