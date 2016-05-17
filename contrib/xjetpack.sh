#!/bin/sh
set -e

devfs_ruleset=5
console=0
console_user=
image=
app_name=the-app

usage () {
    cat >&2 <<EOF
Usage: $0 [FLAGS] DIR
Flags:
  -h           -- show this help
  -d N         -- number or name of devfs ruleset (default: $devfs_ruleset)
  -i IMAGE     -- image to run (only needed first time)
  -c [-u USER] -- run console
  -n NAME      -- app name (default: $app_name)
EOF
    exit ${1:-2}
}

while getopts hcd:i:u:n: opt ; do
    case $opt in
        h) usage 0 ;;
        c) console=1 ;;
        i) image="$OPTARG" ;;
        u) console_user="$OPTARG" ;;
        d) devfs_ruleset="$OPTARG" ;;
        n) app_name="$OPTARG" ;;
        *) usage ;;
    esac
done
shift $(($OPTIND - 1))

test $# -ge 1 || usage

basedir="$1"
xauth_dir="${basedir}/xauth.d"
xauth_fifo="${xauth_dir}/xauthfifo"
manifest_path="${basedir}/manifest.json"
pod_uuid_path="${basedir}/pod.uuid"
image_id_path="${basedir}/image.id"
app_user_path="${basedir}/app_user.txt"

install -d "${basedir}"
install -d -m 0700 "${xauth_dir}"

if ! [ -f "${pod_uuid_path}" ]; then
    if [ "x$image" == "x" ]; then
        echo "ERROR: You need to specify image (-i IMAGE) on the first run" >&2
        usage
    fi

    echo "+++ Creating pod ..."

    manifest="{\"apps\":[{\"name\":\"$app_name\",\"image\":{"
    case "$image" in
        sha512-*)
            manifest="$manifest\"id\": \"$image\""
            ;;
        *)
            image="$(echo "$image" | sed s/:/,version=/)"
            image_name="$(echo "$image," | cut -d, -f1)"
            image_labels="$(echo "$image," | cut -d, -f2-)"
            manifest="$manifest\"name\":\"${image_name}\""
            if [ "x$image_labels" != "x" ]; then
                manifest="${manifest},\"labels\":["
                while true; do
                    current_label="${image_labels%%,*}"
                    image_labels="${image_labels#*,}"
                    current_label_name="${current_label%%=*}"
                    current_label_value="${current_label#*=}"
                    manifest="${manifest}{\"name\":\"${current_label%%=*}\",\"value\":\"${current_label#*=}\"}"
                    if [ "x${image_labels}" = "x" ]; then
                        break
                    else
                        manifest="${manifest},"
                    fi
                done
                manifest="${manifest}]"
            fi
    esac
    manifest="${manifest}},\"mounts\":[{\"volume\":\"x11\",\"path\":\"x11\"},{\"volume\":\"xauth\",\"path\":\"xauth\"}]}],\"volumes\":[{\"name\":\"x11\",\"kind\":\"host\",\"source\":\"/tmp/.X11-unix\"},{\"name\":\"xauth\",\"kind\":\"host\",\"source\":\"${xauth_dir}\"}],\"annotations\":[{\"name\":\"hostname\",\"value\":\"$(hostname)\"},{\"name\":\"jetpack/devfs-ruleset\",\"value\":\"${devfs_ruleset}\"}]}"

    echo "${manifest}" > "${manifest_path}"
    jetpack prepare -saveid="${pod_uuid_path}" -f "${manifest_path}"
    jetpack manifest "$(cat ${pod_uuid_path})" | jq -r '.apps[0].image.id' > "${image_id_path}"
    jetpack image-manifest "$(cat ${image_id_path})" | jq -r '.app.user' > "${app_user_path}"
fi

if [ "${console}" = 1 ]; then
    if [ "x${console_user}" = "x" ]; then
        console_user="$(cat "${app_user_path}")"
    fi
    exec jetpack console -u "${console_user}" "$(cat "${pod_uuid_path}")"
fi

(
    while ! test -p "${xauth_fifo}" ; do
        sleep .2
    done
    set -x
    xauth nextract - $DISPLAY > "${xauth_fifo}"
) &

exec jetpack run "$(cat "${pod_uuid_path}")"
