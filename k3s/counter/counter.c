#include <stdio.h>
#include <unistd.h>
#include <sys/types.h>
#include <stdlib.h>

int main(int argc, char *argv[]) {
    int i = 0;
    pid_t pid = getpid();

    printf("C Counter started. PID: %d\n", pid);
    fflush(stdout);

    while (1) {
        printf("[PID %d] Count: %d\n", pid, i++);
        fflush(stdout);
        sleep(1);
    }

    return 0;
}