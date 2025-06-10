#include <stdio.h>
#include <unistd.h>
#include <sys/types.h>
#include <stdlib.h>
#include <signal.h>

volatile sig_atomic_t stop = 0;

void handle_signal(int sig) {
    printf("\n[PID %d] Received signal %d, exiting...\n", getpid(), sig);
    fflush(stdout);
    stop = 1;
}

int main(int argc, char *argv[]) {
    int i = 0;
    pid_t pid = getpid();

    // register signal handlers
    signal(SIGTERM, handle_signal);
    signal(SIGINT, handle_signal);

    printf("C Counter started. PID: %d\n", pid);
    fflush(stdout);

    while (!stop) {
        printf("[PID %d] Count: %d\n", pid, i++);
        fflush(stdout);
        sleep(1);
    }

    printf("[PID %d] Graceful shutdown complete.\n", pid);
    fflush(stdout);
    return 0;
}
