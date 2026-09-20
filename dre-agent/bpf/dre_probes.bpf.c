//go:build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include "dre_maps.h"
#include "../../api/io_event.h"

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, DRE_RINGBUF_BYTES);
} dre_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u32);
} dre_bypass SEC(".maps");

static __always_inline int dre_is_bypassed(void)
{
    __u32 key = 0;
    __u32 *val = bpf_map_lookup_elem(&dre_bypass, &key);
    return val && *val;
}

static __always_inline void dre_redact_payload(char *payload, __u32 len)
{
    if (len < 7)
        return;
#pragma unroll
    for (int i = 0; i < 48; i++) {
        if ((__u32)(i + 7) > len)
            break;
        if (payload[i] == 'B' && payload[i + 1] == 'e' && payload[i + 2] == 'a' &&
            payload[i + 3] == 'r' && payload[i + 4] == 'e' && payload[i + 5] == 'r' &&
            payload[i + 6] == ' ') {
#pragma unroll
            for (int j = 0; j < 32; j++) {
                if ((__u32)(i + 7 + j) >= len)
                    break;
                payload[i + 7 + j] = '*';
            }
            break;
        }
    }
}

static __always_inline int dre_submit_io_event(void *ctx, __u32 fd, __u32 len,
                                               __u8 is_write, const char *buf)
{
    if (dre_is_bypassed())
        return 0;

    struct dre_io_event *evt = bpf_ringbuf_reserve(&dre_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 payload_len = len;
    if (payload_len > DRE_MAX_PAYLOAD_LEN)
        payload_len = DRE_MAX_PAYLOAD_LEN;
    payload_len &= DRE_MAX_PAYLOAD_LEN;

    evt->pid_tgid = pid_tgid;
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->fd = fd;
    evt->is_write = is_write;
    evt->payload_len = payload_len;
    bpf_get_current_comm(evt->comm, sizeof(evt->comm));

    if (buf && payload_len > 0) {
        if (bpf_probe_read_user(evt->payload, payload_len, buf) != 0)
            evt->payload_len = 0;
        else
            dre_redact_payload(evt->payload, evt->payload_len);
    }

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

SEC("tp/syscalls/sys_enter_read")
int dre_trace_read(struct trace_event_raw_sys_enter *ctx)
{
    __u32 fd = (__u32)ctx->args[0];
    const char *buf = (const char *)ctx->args[1];
    __u32 count = (__u32)ctx->args[2];
    return dre_submit_io_event(ctx, fd, count, 0, buf);
}

SEC("tp/syscalls/sys_enter_write")
int dre_trace_write(struct trace_event_raw_sys_enter *ctx)
{
    __u32 fd = (__u32)ctx->args[0];
    const char *buf = (const char *)ctx->args[1];
    __u32 count = (__u32)ctx->args[2];
    return dre_submit_io_event(ctx, fd, count, 1, buf);
}

SEC("tp/syscalls/sys_enter_recvfrom")
int dre_trace_recvfrom(struct trace_event_raw_sys_enter *ctx)
{
    __u32 fd = (__u32)ctx->args[0];
    const char *buf = (const char *)ctx->args[1];
    __u32 count = (__u32)ctx->args[2];
    return dre_submit_io_event(ctx, fd, count, 0, buf);
}

SEC("tp/syscalls/sys_enter_sendto")
int dre_trace_sendto(struct trace_event_raw_sys_enter *ctx)
{
    __u32 fd = (__u32)ctx->args[0];
    const char *buf = (const char *)ctx->args[1];
    __u32 count = (__u32)ctx->args[2];
    return dre_submit_io_event(ctx, fd, count, 1, buf);
}

SEC("uprobe/clock_gettime")
int dre_uprobe_clock_gettime(struct pt_regs *ctx)
{
    if (dre_is_bypassed())
        return 0;

    struct dre_io_event *evt = bpf_ringbuf_reserve(&dre_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    evt->pid_tgid = bpf_get_current_pid_tgid();
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->fd = (__u32)ctx->rdi;
    evt->is_write = 3; /* clock_gettime marker */
    evt->payload_len = 16;
    bpf_get_current_comm(evt->comm, sizeof(evt->comm));

    struct {
        __s64 tv_sec;
        __s64 tv_nsec;
    } ts = {};
    void *ts_ptr = (void *)ctx->rsi;
    if (ts_ptr) {
        bpf_probe_read_user(&ts, sizeof(ts), ts_ptr);
    }
    __builtin_memcpy(evt->payload, &ts, sizeof(ts));
    bpf_ringbuf_submit(evt, 0);
    return 0;
}

SEC("tp/sched/sched_process_exit")
int dre_trace_process_exit(void *ctx)
{
    if (dre_is_bypassed())
        return 0;

    struct dre_io_event *evt = bpf_ringbuf_reserve(&dre_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    evt->pid_tgid = bpf_get_current_pid_tgid();
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->fd = (__u32)(bpf_get_current_pid_tgid() >> 32);
    evt->is_write = 4; /* process exit marker */
    evt->payload_len = 0;
    bpf_get_current_comm(evt->comm, sizeof(evt->comm));
    bpf_ringbuf_submit(evt, 0);
    return 0;
}

SEC("tp/signal/signal_generate")
int dre_trace_signal_generate(struct trace_event_raw_signal_generate *ctx)
{
    if (dre_is_bypassed())
        return 0;
    if (ctx->sig != 11)
        return 0;

    struct dre_io_event *evt = bpf_ringbuf_reserve(&dre_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    evt->pid_tgid = bpf_get_current_pid_tgid();
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->fd = ctx->sig;
    evt->is_write = 5; /* SIGSEGV marker */
    evt->payload_len = 0;
    bpf_get_current_comm(evt->comm, sizeof(evt->comm));
    bpf_ringbuf_submit(evt, 0);
    return 0;
}

SEC("tp/sched/sched_switch")
int dre_trace_sched_switch(struct trace_event_raw_sched_switch *ctx)
{
    if (dre_is_bypassed())
        return 0;

    struct dre_io_event *evt = bpf_ringbuf_reserve(&dre_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    evt->pid_tgid = bpf_get_current_pid_tgid();
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->fd = 0;
    evt->is_write = 2; /* sched_switch marker */
    evt->payload_len = 0;
    bpf_get_current_comm(evt->comm, sizeof(evt->comm));
    bpf_ringbuf_submit(evt, 0);
    return 0;
}
