package Test::JetpackHelpers;

use autodie qw(:all);
use strict;
use warnings;

use Cwd qw(realpath);
use File::Basename;
use File::Path qw(make_path);
use File::Spec::Functions;

use Test::Command;

BEGIN {
  require Exporter;
  our @ISA = qw(Exporter);
  our @EXPORT = qw(run_command workdir fixture);
}

sub run_command {
  my ( $cmd, $cmdstr, $tcmd );
  if ( $#_ > 0 ) {
    $cmd = \@_;
    $cmdstr = join(' ', @_);
  } else {
    $cmd = $_[0];
    $cmdstr = $cmd;
  }
  $tcmd = Test::Command->new(cmd => $cmd);
  exit_is_num($tcmd, 0, "Run: $cmdstr");
  return $tcmd;
}

my $rootdir = dirname(dirname(dirname(dirname(realpath(__FILE__)))));
my $fixtures = catfile($rootdir, 't', 'fixtures');

sub workdir {
  my $path = catfile($rootdir, 'tmp', 't', basename($0), @_);
  make_path($path);
  return $path;
}

sub fixture {
  my $path = catfile($rootdir, 't', 'fixtures', basename($0), @_);
  die "No such fixture: $path" unless -e $path;
  return $path;
}

1;
