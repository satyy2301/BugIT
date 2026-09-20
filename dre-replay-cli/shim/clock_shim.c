#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdint.h>
#include <time.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int (*real_clock_gettime)(clockid_t, struct timespec *) = NULL;

static void init_real(void) {
    if (!real_clock_gettime) {
        real_clock_gettime = dlsym(RTLD_NEXT, "clock_gettime");
    }
}

static uint64_t read_frozen_ns(void) {
    const char *file = getenv("DRE_FROZEN_TIME_FILE");
    if (file) {
        FILE *f = fopen(file, "r");
        if (f) {
            unsigned long long ts = 0;
            if (fscanf(f, "%llu", &ts) == 1) {
                fclose(f);
                return (uint64_t)ts;
            }
            fclose(f);
        }
    }
    const char *env = getenv("DRE_FROZEN_TIME_NS");
    if (env) {
        return strtoull(env, NULL, 10);
    }
    return 0;
}

int clock_gettime(clockid_t clk_id, struct timespec *tp) {
    init_real();
    uint64_t frozen_ns = read_frozen_ns();
    if (frozen_ns > 0 && tp) {
        tp->tv_sec = (time_t)(frozen_ns / 1000000000ULL);
        tp->tv_nsec = (long)(frozen_ns % 1000000000ULL);
        return 0;
    }
    return real_clock_gettime(clk_id, tp);
}
