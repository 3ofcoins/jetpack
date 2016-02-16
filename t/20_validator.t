# -*- cperl -*-
use warnings;
use strict;
use autodie qw(:all);

use Test::Most tests => 5;
use Test::JetpackHelpers;

use File::Spec::Functions;

use constant APPC_SPEC_BIN => catfile(JETPACK_ROOT, "vendor/github.com/appc/spec/bin");

die_on_fail;

for my $imgname ( qw(ace-validator-main ace-validator-sidekick) ) {
  my $aci = catfile(APPC_SPEC_BIN, "$imgname.aci");
  ok(-f $aci, "have $imgname.aci");
  destroy_images "coreos.com/$imgname";
  run_command 'jetpack', 'import', $aci;
}

run_command(fixture("validate.sh"));
