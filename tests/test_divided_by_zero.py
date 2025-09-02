from divided_by_zero.core import DividedByZeroAgent, DivInput

def test_divide_normal():
    agent = DividedByZeroAgent()
    res = agent.divide(DivInput(a=10, b=2))
    assert res.state == "AFFIRM"

def test_divide_zero():
    agent = DividedByZeroAgent()
    res = agent.divide(DivInput(a=5, b=0))
    assert res.result == 0
    assert res.state == "TEND"
