package Test::JetpackHelpers;

use autodie qw(:all);
use strict;
use warnings;

use Cwd qw(realpath);
use File::Basename;
use File::Path qw(make_path);
use File::Spec::Functions;

use Test::Command;
use Test::Most;

BEGIN {
  require Exporter;
  our @ISA = qw(Exporter);
  our @EXPORT = qw(run_command workdir fixture destroy_images JETPACK_ROOT);
}

use constant JETPACK_ROOT => dirname(dirname(dirname(dirname(realpath(__FILE__)))));

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

sub workdir {
  my $path = catfile(JETPACK_ROOT, 'tmp', 't', basename($0), @_);
  make_path($path);
  return $path;
}

sub fixture {
  my $path = catfile(JETPACK_ROOT, 't', 'fixtures', basename($0), @_);
  die "No such fixture: $path" unless -e $path;
  return $path;
}

sub destroy_images {
  chomp(my $imgs=`jetpack images -H -l`);
  die "can't list images: $?" if $? > 0;
  my @imgs = split "\n", $imgs;

  foreach my $wanted ( @_ ) {
    for ( @imgs ) {
      my ($id, $name) = split("\t", $_);
      my $shortname = $name;
      $shortname =~ s/[:,].*$//;
      next unless $shortname eq $wanted;
      note("Destroying image $id ($name)\n");
      system "jetpack destroy-image $id >/dev/null 2>&1"
    }
  }
}

1;
