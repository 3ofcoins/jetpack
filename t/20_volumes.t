# -*- cperl -*-
use warnings;
use strict;
use autodie qw(:all);

use Test::Most tests => 2;
use Test::JetpackHelpers;

use File::Slurp;
use File::Spec::Functions;

die_on_fail;

my $idfile = catfile(workdir, "pod.id");

run_command('jetpack', 'prepare', '-saveid', $idfile, '-f', fixture("pod-manifest.json"));
chomp(my $pod = read_file($idfile));
run_command('jetpack', 'run', '-destroy', $pod);
