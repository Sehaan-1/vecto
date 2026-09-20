from data_processor import process_data

def test_process_data():
    res = process_data(5)
    assert res == [0, 2, 4, 6, 8], f"unexpected result: {res}"
    print("Python processor tests passed")

if __name__ == "__main__":
    test_process_data()
