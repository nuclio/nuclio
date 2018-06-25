#!/usr/bin/env node
// Install packages specified in package.json (from stdin) globally

/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

const { spawnSync } = require('child_process');

var input = '';

process.stdin.resume();
process.stdin.setEncoding('utf8');

process.stdin.on('data', function (chunk) {
    input += chunk;
});

process.stdin.on('end', function () {
    var obj = JSON.parse(input);

    for (var name in obj.dependencies) {
	var dep = name + '@' + obj.dependencies[name];
	var cmd = ['npm', 'install', '-g', dep];
	console.log(cmd.join(' '));
	var out = spawnSync(
	    cmd[0],
	    cmd.slice(1),
	    {stdio: ['inherit', 'inherit', 'inherit']}
	);

	if (out.status != 0) {
	    process.exit(1);
	}
    }
});
