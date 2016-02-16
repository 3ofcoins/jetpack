# -*- cperl -*-

use warnings;
use strict;
use autodie qw(:all);

use Test::Most tests => 14;
use Test::Command;

# TODO: this will work on amd64 only
# TODO: maybe we shouldn't check exact version/checksum of freebsd-base image? This changes quite often…

# Parameters, may need to be updated
my $signing_key_fingerprint = '4706dc5d5c214bc3ad127c6d53ccc2d63a162664';
my $base_image_version = '10.2.8';
my $base_image_id = 'sha512-330aaaa197708a1e8945769f7b13b91e5b7113981bdfd1838380378e7528e837abebd8de3923a5450425c9c1a8f2b71e164a8abbcbf2bd0f55b9ebc4a22cf0a7';

my %datasets;
sub reload_datasets {
  %datasets = ();
  foreach (split "\n", `zfs list -H`) {
    my @fields = split "\t", $_;
    $datasets{$fields[0]} = $fields[$#fields];
  }
}

# This is initialization & sanity checks. If any check of this script
# fails, we're in no condition to run other tests.
bail_on_fail;

# Check root ZFS dataset name
my $cmd = Test::Command->new(cmd => 'jetpack config root.zfs');
exit_is_num($cmd, 0, "Can get root ZFS dataset name");
chomp(my $rootds = stdout_value($cmd));

# Make sure root dataset does not exist
reload_datasets;
ok(!defined($datasets{$rootds}), "Root dataset does not exist");

# Initialize Jetpack
exit_is_num('jetpack init', 0, "Datasets have been initialized");

# See that the datasets do exist now
reload_datasets;
ok(defined($datasets{$rootds}), "Root dataset has been created");
ok(defined($datasets{$rootds}."/images"), "Images dataset has been created");
ok(defined($datasets{$rootds}."/pods"), "Pods dataset has been created");

# Trust fingerprint
exit_is_num("jetpack trust -fingerprint=$signing_key_fingerprint 3ofcoins.net", 0, "Imported 3ofcoins.net signing key");

# Check that fingerprint is trusted
$cmd = Test::Command->new(cmd => 'jetpack trust');
exit_is_num($cmd, 0, "Listed trusted fingerprints");
my @lines = split "\n", stdout_value($cmd);
ok($#lines==1, "There is one trusted key");
ok($lines[1] =~ /^3ofcoins\.net\s+$signing_key_fingerprint\s+/, "The 3ofcoins.net key is trusted");

# Fetch base image
exit_is_num('jetpack fetch 3ofcoins.net/freebsd-base', 0, "Fetched freebsd-base image");
$cmd = Test::Command->new(cmd => 'jetpack images -H -l');
exit_is_num($cmd, 0, "Listed images");
@lines = split "\n", stdout_value($cmd);
ok($#lines == 0, "There is only one image");
ok($lines[0] eq "$base_image_id\t3ofcoins.net/freebsd-base:$base_image_version,arch=\"amd64\",os=\"freebsd\"", "The image ID, name, and tags are what we expected");
