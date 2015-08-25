#include <sys/param.h>
#include <sys/jail.h>

#include <err.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static char *argv0;

void usage()
{
     fprintf(stderr, "Usage: %s JID:UID:GID:APP:CWD [VAR=val...] PROG ARG...\n", argv0);
     exit(1);
}

int main(int argc, char *argv[])
{
     int jid, i;
     uid_t uid;
     gid_t gid;
     char *cur, *next, *endp, *app, *cwd, *rootdir, **eargv, **eenvp;

     argv0 = argv[0];           /* for usage() */
     if ( !(argc>=3) ) {
          usage();
     }

     /* 
      * Command line processing
      */

     /* Split argv[1] into JID:UID:GID:APP:CWD */
     next = argv[1];
#define _step() if ( !((cur = strsep(&next, ":")) && *cur) ) { usage(); }

     _step();
     jid = strtol(cur, &endp, 10);
     if ( *endp ) {
          usage();
     }

     _step();
     uid = strtol(cur, &endp, 10);
     if ( *endp ) {
          usage();
     }

     _step();
     gid = strtol(cur, &endp, 10);
     if ( *endp ) {
          usage();
     }

     _step();
     app = cur;

     if ( !(*next) ) {
          usage();
     }

     cwd = next;

     /* Biggest possible envp */
     if ( !(eenvp = calloc(argc-1, sizeof(char*))) ) {
          err(1, "calloc");
     }

     i = strlen(app)+sizeof("AC_APP_NAME=*");
     if ( !(eenvp[0] = malloc(i)) ) {
          err(1, "malloc");
     }

     if ( snprintf(eenvp[0], i, "AC_APP_NAME=%s", app) < 0 ) {
          err(1, "snprintf");
     }

     /* Copy argv to envp until we meet a path or run out of argv */
     for ( i=2 ; i < argc && argv[i] && argv[i][0] != '/' ; i++ ) {
          eenvp[i-1] = argv[i];
     }

     /* If we ran out of argv, bomb. */
     if ( i == argc ) {
          usage();
     }

     /* Rest of our argv is exec's argv */
     eargv = argv + i;

     /*
      * Actual isolation
      */

     if ( jail_attach(jid) < 0 ) {
          err(1, "jail_attach(%d)", jid);
     }

     i = strlen(app) + sizeof("/app/*/rootfs");
     if ( !(rootdir = malloc(i)) ) {
          err(1, "malloc");
     }
     if ( snprintf(rootdir, i, "/app/%s/rootfs", app) < 0 ) {
          err(1, "snprintf");
     }

     if ( chroot(rootdir) < 0 ) {
          err(1, "chroot: %s", rootdir);
     }

     if ( chdir(cwd) < 0 ) {
          err(1, "chdir: %s", cwd);
     }

     if ( setgroups(1, &gid) < 0 ) {
          err(1, "setgroups: %d", gid);
     }

     if ( setgid(gid) < 0 ) {
          err(1, "setgid: %d", gid);
     }

     if ( setuid(uid) < 0 ) {
          err(1, "setuid: %d", uid);
     }

     /* 
      * Exec the target command
      */

     execve(eargv[0], eargv, eenvp);
     err(1, "execve: %s", eargv[0]);

     return 0;
}
