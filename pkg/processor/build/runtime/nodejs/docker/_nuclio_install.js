#!/usr/bin/env node
// Install packages specified in package.json (from stdin) globally

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
