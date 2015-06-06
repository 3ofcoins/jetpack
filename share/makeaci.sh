#!/bin/sh

if [ $# -ne 3 ]; then
    echo "Usage: $0 path/to/rootfs.tgz path/to/manifest.json path/to/output.aci" >&2
    exit 1
fi

if ! which fakeroot >/dev/null ; then
    echo "Please install security/fakeroot to use this script" >&2
    exit 1
fi

pv=$(which pv || echo cat)
tmpdir=$(mktemp -d ./makeaci.XXXXXXXX)
savefile=${tmpdir}/fakeroot.save

fr ()
{
    case $1 in
        [\<\>\|]*)
            suffix=" $1"
            shift
            ;;
        *)
            suffix=""
    esac

    echo "# $*$suffix" >&2
    if test -f ${savefile} ; then load="-i ${savefile}" ; else load='' ; fi
    fakeroot -s ${savefile} ${load} -- "${@}"
}

fr install -v -d -o 0 -g 0 -m 0755 $tmpdir/rootfs
fr install -v -o 0 -g 0 -m 0444 $2 $tmpdir/manifest
${pv} "$1" | fr "< $1" tar -C $tmpdir/rootfs -xf -
fr "| xz > $3" tar -C $tmpdir -cf - manifest rootfs | xz -z -c | ${pv} > "$3"

rm -rf ${tmpdir}
