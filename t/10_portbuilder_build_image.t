# -*- cperl -*-

# Needs: freebsd-base image
# Removes preexisting portbuilder images.

use warnings;
use strict;
use autodie qw(:all);

use Test::Most tests => 14;
use Test::Command;
use Test::JetpackHelpers;

use Cwd;
use File::Basename;
use File::Path qw(make_path);
use File::Slurp;

die_on_fail;

# Check prerequisites
run_command 'jetpack', 'show-image', '3ofcoins.net/freebsd-base';

destroy_images "3ofcoins.net/port-builder";

my $builddir = dirname(dirname(Cwd::realpath(__FILE__)))."/images/portbuilder";

# Build the image
run_command "make", "-C", $builddir, "clean";
run_command "make", "-C", $builddir;

# Make sure it's built
ok(-f "$builddir/image.aci.id", "Got image ID");
chomp(my $imgid = read_file("$builddir/image.aci.id"));

# Check the image's parameters
my $cmdout = stdout_value run_command "jetpack", "show-image", $imgid;
ok($cmdout =~ /^ID\s+$imgid$/m, "Image ID matches");
ok($cmdout =~ /^Name\s+3ofcoins\.net\/port-builder:\d+\.\d+\.\d+,arch=amd64,os=freebsd$/m, "Image name matches");
ok($cmdout =~ /^Dependencies\s+sha512-[0-9a-f]+ 3ofcoins\.net\/freebsd-base:10\.\d+\.\d+/m, "Dependency on freebsd-base");

# Export the image (via make)
run_command "make", "-C", $builddir, "aci";
ok(-f "$builddir/image.aci", "ACI file exists");
run_command "actool", "validate", "$builddir/image.aci";

run_command "make", "-C", $builddir, "flat-aci";
ok(-f "$builddir/image.flat.aci", "Flat ACI file exists");
run_command "actool", "validate", "$builddir/image.flat.aci";
