#ifndef DRE_IO_EVENT_H
#define DRE_IO_EVENT_H

#define DRE_MAX_PAYLOAD_LEN 2048
#define DRE_COMM_LEN 16

/* PRD §5.1 — kernel event binary structure */
struct io_event {
    __u64 pid_tgid;        /* PID (top 32) + TID (bottom 32) */
    __u64 timestamp_ns;    /* bpf_ktime_get_ns() */
    __u32 fd;
    __u32 payload_len;
    __u8  is_write;        /* 0 = read/recv, 1 = write/send */
    char  comm[DRE_COMM_LEN];
    char  payload[DRE_MAX_PAYLOAD_LEN];
};

#define DRE_IO_EVENT_SIZE (sizeof(struct io_event))

#endif /* DRE_IO_EVENT_H */
