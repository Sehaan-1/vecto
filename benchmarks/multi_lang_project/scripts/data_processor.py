import sys

def process_data(count=10):
    return [x * 2 for x in range(count)]

if __name__ == "__main__":
    data = process_data()
    print(f"Processed {len(data)} records: {data}")
