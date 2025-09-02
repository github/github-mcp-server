import argparse, json
from .core import DividedByZeroAgent, DivInput

def main():
    p = argparse.ArgumentParser(prog="divided-by-zero")
    sub = p.add_subparsers(dest="cmd", required=True)

    run = sub.add_parser("run")
    run.add_argument("--a", type=float, required=True)
    run.add_argument("--b", type=float, required=True)

    def _run(args):
        agent = DividedByZeroAgent()
        inp = DivInput(a=args.a, b=args.b)
        res = agent.divide(inp)
        print(json.dumps(res.model_dump(), indent=2))

    run.set_defaults(func=_run)

    args = p.parse_args()
    args.func(args)

if __name__ == "__main__":
    main()
