SENT_MESSAGE = "Sent message"
RECEIVED_MESSAGE = "Received message"
TRANSACTION_INIT = "Initialising transaction"
TRANSACTION_COMMIT = "Delivered transaction"

sent_messages = {}
received_messages = {}
transaction_inits = {}
transaction_commit_infos = {}


class TransactionInfo:
    def __init__(self, received_messages_cnt, commit_timestamp):
        self.received_messages_cnt = received_messages_cnt
        self.commit_timestamp = commit_timestamp


n = int(input())


def process_file(file):
    f = open(file, "r")
    for line in f:
        if SENT_MESSAGE in line:
            start = line.find(SENT_MESSAGE)
            data = list(map(lambda elem: elem.split(': ')[1], line[start::].split(', ')))
            sent_messages[data[0]] = int(data[1])
        elif RECEIVED_MESSAGE in line:
            start = line.find(RECEIVED_MESSAGE)
            data = list(map(lambda elem: elem.split(': ')[1], line[start::].split(', ')))
            received_messages[data[0]] = int(data[1])
        elif TRANSACTION_INIT in line:
            start = line.find(TRANSACTION_INIT)
            data = list(map(lambda elem: elem.split(': ')[1], line[start::].split(', ')))
            transaction_inits[data[0]] = int(data[1])
        elif TRANSACTION_COMMIT in line:
            start = line.find(TRANSACTION_COMMIT)
            data = list(map(lambda elem: elem.split(': ')[1], line[start::].split(', ')))
            received_messages_cnt = int(data[2])
            commit_timestamp = int(data[3])
            if transaction_commit_infos.get(data[0]) is None:
                transaction_commit_infos[data[0]] = []
            transaction_commit_infos[data[0]].append(TransactionInfo(received_messages_cnt, commit_timestamp))


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
        commit_info = transaction_commit_infos[transaction]

        if commit_info is None or len(commit_info) != n:
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


files = []

for i in range(n):
    files.append(f"process{i}.txt")

for file in files:
    process_file(file)

min_message_latency, max_message_latency, avg_message_latency = calc_message_latencies()
min_transaction_latency, max_transaction_latency, avg_transaction_latency, avg_messages_exchanged = \
    calc_transaction_stat()

print("Message latencies:")
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
