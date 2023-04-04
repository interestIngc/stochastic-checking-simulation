#!/usr/bin/env python3

SENT_MESSAGE = "Sent message"
RECEIVED_MESSAGE = "Received message"
TRANSACTION_INIT = "Initialising transaction"
TRANSACTION_COMMIT = "Delivered transaction"
WITNESS_SET_SELECTED = "Witness set selected"
PROCESS_STARTED = "Process started"
WITNESS_SET_SELECTION = "Witness set selection"

RELIABLE_ACCOUNTABILITY = "reliable_accountability"
CONSISTENT_ACCOUNTABILITY = "consistent_accountability"


class TransactionInfo:
    def __init__(self, received_messages_cnt, commit_timestamp):
        self.received_messages_cnt = received_messages_cnt
        self.commit_timestamp = commit_timestamp


sent_messages = {}
received_messages = {}
transaction_inits = {}
transaction_commit_infos = {}
transaction_histories = {}
transaction_witness_sets = {
    "own": {},
    "pot": {}
}
protocol = ""

n = int(input())


def parse_data_from_logged_line(line, start_word):
    return list(map(
        lambda elem: elem.split(': ')[1],
        line[line.find(start_word)::].split(', ')
    ))


def process_file(file):
    global protocol
    f = open(file, "r")
    for line in f:
        line = line.strip(" \n")
        if PROCESS_STARTED in line:
            _, protocol = parse_data_from_logged_line(line, PROCESS_STARTED)
        elif SENT_MESSAGE in line:
            data = parse_data_from_logged_line(line, SENT_MESSAGE)
            sent_messages[data[0]] = int(data[1])
        elif RECEIVED_MESSAGE in line:
            data = parse_data_from_logged_line(line, RECEIVED_MESSAGE)
            received_messages[data[0]] = int(data[1])
        elif TRANSACTION_INIT in line:
            data = parse_data_from_logged_line(line, TRANSACTION_INIT)
            transaction_inits[data[0]] = int(data[1])
        elif TRANSACTION_COMMIT in line:
            data = parse_data_from_logged_line(line, TRANSACTION_COMMIT)
            transaction = data[0]
            received_messages_cnt = int(data[2])
            commit_timestamp = int(data[3])
            if transaction_commit_infos.get(transaction) is None:
                transaction_commit_infos[transaction] = []
            transaction_commit_infos[transaction].append(TransactionInfo(received_messages_cnt, commit_timestamp))
        elif WITNESS_SET_SELECTION in line:
            data = parse_data_from_logged_line(line, WITNESS_SET_SELECTION)
            transaction = data[0]

            assert data[2][0] == '[' and data[2][-1] == ']'

            history_str = data[2][1:-1]
            history = set()
            if len(history_str) != 0:
                history = set(history_str.split(' '))

            if transaction_histories.get(transaction) is None:
                transaction_histories[transaction] = []
            transaction_histories[transaction].append(history)
        elif WITNESS_SET_SELECTED in line:
            data = parse_data_from_logged_line(line, WITNESS_SET_SELECTED)
            ws_type = data[0]
            transaction = data[1]
            pids_str = data[2]

            assert pids_str[0] == '[' and pids_str[-1] == ']'
            pids = set(pids_str[1:-1].split(' '))

            if transaction_witness_sets[ws_type].get(transaction) is None:
                transaction_witness_sets[ws_type][transaction] = []
            transaction_witness_sets[ws_type][transaction].append(pids)


def calc_message_latencies():
    latencies = []
    for message, send_timestamp in sent_messages.items():
        receive_timestamp = received_messages.get(message)
        if receive_timestamp is None:
            continue
        latency = receive_timestamp - send_timestamp
        latencies.append(latency)

    min_latency, max_latency, sum_latency = latencies[0], latencies[0], 0

    for latency in latencies:
        min_latency = min(min_latency, latency)
        max_latency = max(max_latency, latency)
        sum_latency += latency

    return min_latency, max_latency, sum_latency / len(latencies)


def calc_transaction_stat():
    sum_latency = 0
    min_latency = None
    max_latency = None
    sum_messages_exchanged = 0
    transaction_cnt = 0
    for transaction, init_timestamp in transaction_inits.items():
        commit_info = transaction_commit_infos.get(transaction)
        if commit_info is None:
            print(f"Transaction {transaction} was not committed")
            continue

        if len(commit_info) != n:
            print(f"Transaction {transaction} wasn't committed by all the processes")
            continue

        transaction_cnt += 1
        max_commit_timestamp = -1
        for transaction_info in commit_info:
            max_commit_timestamp = max(max_commit_timestamp, transaction_info.commit_timestamp)
            sum_messages_exchanged += transaction_info.received_messages_cnt

        latency = max_commit_timestamp - init_timestamp

        if min_latency is None or latency < min_latency:
            min_latency = latency
        if max_latency is None or latency > max_latency:
            max_latency = latency
        sum_latency += latency

    return min_latency, max_latency, sum_latency / transaction_cnt, int(sum_messages_exchanged / transaction_cnt)


def get_distance_metrics(sets):
    union_set = sets[0]
    intersection_set = sets[0]
    for i in range(1, len(sets)):
        union_set = union_set.union(sets[i])
        intersection_set = intersection_set.intersection(sets[i])
    return len(union_set) - len(intersection_set)


def get_witness_sets_diff_metrics(ws_type):
    metrics = []
    for transaction, witness_sets in transaction_witness_sets[ws_type].items():
        if len(witness_sets) != n:
            continue
        metrics.append(get_distance_metrics(witness_sets))

    return metrics


def get_histories_diff_metrics():
    metrics = []
    for transaction, histories in transaction_histories.items():
        if len(histories) != n:
            continue
        metrics.append(get_distance_metrics(histories))

    return metrics


files = []

for i in range(n):
    files.append(f"outputs/process{i}.txt")

for file in files:
    process_file(file)

min_message_latency, max_message_latency, avg_message_latency = calc_message_latencies()
min_transaction_latency, max_transaction_latency, avg_transaction_latency, avg_messages_exchanged = \
    calc_transaction_stat()

print("Message latencies:")
print(f"\tMinimal: {min_message_latency / 1e6}")
print(f"\tMaximal: {max_message_latency / 1e6}")
print(f"\tAverage: {avg_message_latency / 1e6}")
print()

print("Transaction latency statistics:")
print(f"\tMinimal: {min_transaction_latency / 1e6}")
print(f"\tMaximal: {max_transaction_latency / 1e6}")
print(f"\tAverage: {avg_transaction_latency / 1e6}")
print()

print(f"Average number of exchanged messages per one transaction: {avg_messages_exchanged}")
print()

if protocol in {CONSISTENT_ACCOUNTABILITY, RELIABLE_ACCOUNTABILITY}:
    own_witness_sets_diff_metrics = get_witness_sets_diff_metrics(ws_type="own")
    print(f"Difference metrics for own witness sets: {own_witness_sets_diff_metrics}")
    print()

    if protocol == RELIABLE_ACCOUNTABILITY:
        pot_witness_sets_diff_metrics = get_witness_sets_diff_metrics(ws_type="pot")
        print(f"Difference metrics for pot witness sets: {pot_witness_sets_diff_metrics}")
        print()

    histories_diff_metrics = get_histories_diff_metrics()
    print(f"Difference metrics of histories: {histories_diff_metrics}")
    print()
