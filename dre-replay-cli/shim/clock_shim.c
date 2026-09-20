#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdint.h>
#include <time.h>
#include <stdio.h>
#include <stdlib.h>

static int (*real_clock_gettime)(clockid_t, struct timespec *) = NULL;
static uint64_t frozen_ns = 0;

static void init_real(void) {
    if (!real_clock_gettime) {
        real_clock_gettime = dlsym(RTLD_NEXT, "clock_gettime");
    }
}

int clock_gettime(clockid_t clk_id, struct timespec *tp) {
    init_real();
    const char *env = getenv("DRE_FROZEN_TIME_NS");
    if (env && tp) {
        frozen_ns = strtoull(env, NULL, 10);
        tp->tv_sec = (time_t)(frozen_ns / 1000000000ULL);
        tp->tv_nsec = (long)(frozen_ns % 1000000000ULL);
        return 0;
    }
    return real_clock_gettime(clk_id, tp);
}
