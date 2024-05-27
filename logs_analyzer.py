#!/usr/bin/env python3
import sys

SENT_MESSAGE = "Sent message"
RECEIVED_MESSAGE = "Received message"
TRANSACTION_INIT = "Initialising transaction"
TRANSACTION_COMMIT = "Delivered transaction"
WITNESS_SET_SELECTED = "Witness set selected"
WITNESS_SET_SELECTION = "Witness set selection"
SIMULATION_STARTED = "Simulation started"

RELIABLE_ACCOUNTABILITY = "reliable_accountability"
CONSISTENT_ACCOUNTABILITY = "consistent_accountability"

LOG_PREFIXES = {
    SENT_MESSAGE,
    RECEIVED_MESSAGE,
    TRANSACTION_INIT,
    TRANSACTION_COMMIT,
    WITNESS_SET_SELECTED,
    WITNESS_SET_SELECTION,
    SIMULATION_STARTED
}


class TransactionInitInfo:
    def __init__(self, process_id, init_timestamp):
        self.process_id = process_id
        self.init_timestamp = init_timestamp


class TransactionCommitInfo:
    def __init__(self, process_id, received_messages_cnt, commit_timestamp):
        self.process_id = process_id
        self.received_messages_cnt = received_messages_cnt
        self.commit_timestamp = commit_timestamp


def drop_date(line):
    start = 0
    while start < len(line):
        if line[start].isalpha():
            break
        start += 1
    return line[start::]


def parse_data_from_logged_line(line):
    return list(map(
        lambda elem: elem.split(': ')[1],
        line.split(', ')
    ))


def get_log_line_prefix(line):
    prefix = ""
    for log_prefix in LOG_PREFIXES:
        if line.startswith(log_prefix):
            prefix = log_prefix
            break
    return prefix


def parse_data_from_files(directory, n):
    sent_messages = {}
    received_messages = {}
    transaction_inits = {}
    transaction_commit_infos = {}
    transaction_histories = {}
    transaction_witness_sets = {
        "own": {},
        "pot": {}
    }
    simulation_start = None
    simulation_end = None

    for process_id in range(n):
        f = open(f"{directory}/process{process_id}.txt", "r")
        for line in f:
            line = drop_date(line.strip(" \n"))
            prefix = get_log_line_prefix(line)

            if prefix == "":
                continue

            data = parse_data_from_logged_line(line)
            timestamp = int(data[-1])
            if simulation_end is None or timestamp > simulation_end:
                simulation_end = timestamp

            if prefix == SIMULATION_STARTED:
                if simulation_start is None or timestamp < simulation_start:
                    simulation_start = timestamp
            elif prefix == SENT_MESSAGE:
                sent_messages[data[0]] = timestamp
            elif prefix == RECEIVED_MESSAGE:
                received_messages[data[0]] = timestamp
            elif prefix == TRANSACTION_INIT:
                transaction_inits[data[0]] = \
                    TransactionInitInfo(process_id=process_id, init_timestamp=timestamp)
            elif prefix == TRANSACTION_COMMIT:
                transaction = data[0]
                received_messages_cnt = int(data[2])
                if transaction_commit_infos.get(transaction) is None:
                    transaction_commit_infos[transaction] = []
                transaction_commit_infos[transaction].append(
                    TransactionCommitInfo(
                        process_id=process_id,
                        received_messages_cnt=received_messages_cnt,
                        commit_timestamp=timestamp)
                )
            elif prefix == WITNESS_SET_SELECTION:
                transaction = data[0]

                assert data[2][0] == '[' and data[2][-1] == ']'

                history_str = data[2][1:-1]
                history = set()
                if len(history_str) != 0:
                    history = set(history_str.split(' '))

                if transaction_histories.get(transaction) is None:
                    transaction_histories[transaction] = []
                transaction_histories[transaction].append(history)
            elif prefix == WITNESS_SET_SELECTED:
                ws_type = data[0]
                transaction = data[1]

                assert data[2][0] == '[' and data[2][-1] == ']'

                pids_str = data[2][1:-1]
                pids = set()
                if len(pids_str) != 0:
                    pids = set(pids_str.split(' '))

                if transaction_witness_sets[ws_type].get(transaction) is None:
                    transaction_witness_sets[ws_type][transaction] = []
                transaction_witness_sets[ws_type][transaction].append(pids)
    return {
        "sent_messages": sent_messages,
        "received_messages": received_messages,
        "transaction_inits": transaction_inits,
        "transaction_commit_infos": transaction_commit_infos,
        "transaction_histories": transaction_histories,
        "transaction_witness_sets": transaction_witness_sets,
        "simulation_start": simulation_start,
        "simulation_end": simulation_end
    }


def calc_message_latencies(sent_messages, received_messages):
    latencies = []
    lost_messages_cnt = 0
    for message, send_timestamp in sent_messages.items():
        receive_timestamp = received_messages.get(message)
        if receive_timestamp is None:
            lost_messages_cnt += 1
            continue
        latencies.append(receive_timestamp - send_timestamp)

    if len(latencies) == 0:
        return 0, 0, 0

    min_latency, max_latency, sum_latency = latencies[0], latencies[0], 0

    for latency in latencies:
        min_latency = min(min_latency, latency)
        max_latency = max(max_latency, latency)
        sum_latency += latency

    return lost_messages_cnt, len(sent_messages), min_latency, max_latency, sum_latency / len(latencies)


def calc_transaction_stat(n, transaction_inits, transaction_commit_infos, simulation_start, simulation_end):
    sum_latency = 0
    min_latency = None
    max_latency = None
    sum_messages_exchanged = 0
    transaction_cnt = 0
    for transaction, init_info in transaction_inits.items():
        commit_infos = transaction_commit_infos.get(transaction)
        if commit_infos is None:
            print(f"Transaction {transaction} was not committed")
            continue

        if len(commit_infos) != n:
            committed_pids = set(map(lambda commit_info: commit_info.process_id, commit_infos))
            not_committed_pids = set(range(n)).difference(committed_pids)
            print(f"Transaction {transaction} wasn't committed by processes {not_committed_pids}")

        commit_timestamp = None
        messages_exchanged = 0
        for commit_info in commit_infos:
            if commit_info.process_id == init_info.process_id:
                commit_timestamp = commit_info.commit_timestamp
            messages_exchanged += commit_info.received_messages_cnt

        if commit_timestamp is None:
            print(f"Transaction {transaction} wasn't committed by source")
            continue

        latency = commit_timestamp - init_info.init_timestamp

        if min_latency is None or latency < min_latency:
            min_latency = latency
        if max_latency is None or latency > max_latency:
            max_latency = latency
        sum_latency += latency

        sum_messages_exchanged += messages_exchanged
        transaction_cnt += 1

    throughput = 0
    if simulation_start is not None and simulation_end is not None:
        throughput = transaction_cnt * 1e9 / (simulation_end - simulation_start)
    return min_latency, max_latency, sum_latency / transaction_cnt, \
        int(sum_messages_exchanged / transaction_cnt), throughput


def get_distance_metrics(sets):
    max_diff = 0
    for i in range(len(sets)):
        for j in range(i + 1, len(sets)):
            intersection_size = len(sets[i].intersection(sets[j]))
            union_size = len(sets[i].union(sets[j]))
            max_diff = max(max_diff, union_size - intersection_size)

    return max_diff


def get_witness_sets_diff_metrics(transaction_witness_sets, n, ws_type):
    metrics = []
    for transaction, witness_sets in transaction_witness_sets[ws_type].items():
        if len(witness_sets) != n:
            continue
        metrics.append(get_distance_metrics(witness_sets))

    return metrics


def get_histories_diff_metrics(transaction_histories, n):
    metrics = []
    for transaction, histories in transaction_histories.items():
        if len(histories) != n:
            continue
        metrics.append(get_distance_metrics(histories))

    return metrics


def calculate_stat(directory, n):
    data = parse_data_from_files(directory, n)

    lost_messages_cnt, total_messages_cnt, min_message_latency, max_message_latency, avg_message_latency = \
        calc_message_latencies(
            sent_messages=data["sent_messages"],
            received_messages=data["received_messages"]
        )
    min_transaction_latency, max_transaction_latency, avg_transaction_latency, avg_messages_exchanged, throughput = \
        calc_transaction_stat(
            n=n,
            transaction_inits=data["transaction_inits"],
            transaction_commit_infos=data["transaction_commit_infos"],
            simulation_start=data["simulation_start"],
            simulation_end=data["simulation_end"]
        )

    return {
        "lost_messages_cnt": lost_messages_cnt,
        "total_messages_cnt": total_messages_cnt,
        "min_message_latency": min_message_latency / 1e9,
        "max_message_latency": max_message_latency / 1e9,
        "avg_message_latency": avg_message_latency / 1e9,
        "min_transaction_latency": min_transaction_latency / 1e9,
        "max_transaction_latency": max_transaction_latency / 1e9,
        "avg_transaction_latency": avg_transaction_latency / 1e9,
        "avg_messages_exchanged": avg_messages_exchanged,
        "throughput": throughput,
    }


if __name__ == "__main__":
    process_cnt = int(sys.argv[1])
    output_dir = sys.argv[2]

    print(f"Calculating statistics for {process_cnt} processes")
    print()

    stat = calculate_stat(directory=output_dir, n=process_cnt)

    lost_messages_cnt, total_messages_cnt, min_message_latency, max_message_latency, avg_message_latency = \
        stat["lost_messages_cnt"], stat["total_messages_cnt"], \
        stat["min_message_latency"], stat["max_message_latency"], stat["avg_message_latency"]
    min_transaction_latency, max_transaction_latency, avg_transaction_latency, avg_messages_exchanged, throughput = \
        stat["min_transaction_latency"], stat["max_transaction_latency"], \
        stat["avg_transaction_latency"], stat["avg_messages_exchanged"], stat["throughput"]

    print("Messages statistics:")
    print(f"\tMessages lost: {lost_messages_cnt}")
    print(f"\tMessages total: {total_messages_cnt}")
    print(f"\tMinimal: {min_message_latency}")
    print(f"\tMaximal: {max_message_latency}")
    print(f"\tAverage: {avg_message_latency}")
    print()

    print("Transaction latency statistics:")
    print(f"\tMinimal: {min_transaction_latency}")
    print(f"\tMaximal: {max_transaction_latency}")
    print(f"\tAverage: {avg_transaction_latency}")
    print()

    print(f"Average number of exchanged messages per one transaction: {avg_messages_exchanged}")
    print()

    print(f"Throughput per second: {throughput}")
    print()
