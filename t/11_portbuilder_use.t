# -*- cperl -*-

use warnings;
use strict;
use autodie qw(:all);

use File::Slurp;
use File::Spec::Functions;

use Test::Most tests => 6;
use Test::Command;
use Test::JetpackHelpers;

# Check prerequisites
die_on_fail;
ok(-f '/usr/ports/misc/figlet/Makefile', "have ports tree");
run_command('jetpack', 'show-image', '3ofcoins.net/port-builder');
restore_fail;

my $pkgdir = workdir('vol.packages');
my $idfile = catfile(workdir, "pod.id");
run_command 'jetpack', 'prepare', "-saveid=$idfile", '--',
  '-v', 'ports:/usr/ports',
  '-v', 'distfiles:/usr/ports/distfiles',
  '-v', "packages:$pkgdir",
  '3ofcoins.net/port-builder',
  '-a', 'port=misc/figlet',
  '-a', 'make=package install';

chomp(my $pod = read_file($idfile));
run_command 'jetpack', 'run', $pod;

ok(glob("$pkgdir/All/figlet-*.txz"), "figlet package has been built");

# TODO: {
#   local $TODO = '`jetpack enter` not implemented yet';
#   my $cmdout = stdout_value run_command "jetpack", "enter", $pod, "figlet", "testink";
#   ok($cmdout eq <<EOF, "figlet works in pod");
#  _            _   _       _    
# | |_ ___  ___| |_(_)_ __ | | __
# | __/ _ \/ __| __| | '_ \| |/ /
# | ||  __/\__ \ |_| | | | |   < 
#  \__\___||___/\__|_|_| |_|_|\_\
                               
# EOF
# }

run_command 'jetpack', 'destroy', $pod;
