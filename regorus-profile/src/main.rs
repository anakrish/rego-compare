#[derive(clap::Parser)]
#[command(author, version, about, long_about = None)]
struct Cli {
    /// Policy or data files. Rego, json or yaml.
    #[arg(long, short, value_name = "policy.rego|data.json")]
    data: Vec<String>,

    /// Input file. json or yaml.
    #[arg(long, short, value_name = "input.rego")]
    input: Option<String>,

    /// Number of iterations.
    #[arg(long, short, value_name = "num-iterations", default_value = "100000")]
    num_iterations: usize,

    /// Rego rule.
    #[arg(long, short, value_name = "rule")]
    rule: Option<String>,

    /// Rego query
    #[arg(long, short, value_name = "query")]
    query: Option<String>,

    // Show output
    #[arg(long, short, value_name = "show-output", default_value = "false")]
    show_output: bool,
}

fn main() {
    use clap::Parser;
    let cli = Cli::parse();

    let mut engine = regorus::Engine::new();
    for d in cli.data {
        if d.ends_with(".rego") {
            engine
                .add_policy_from_file(&d)
                .expect(&format!("failed to add policy from {d}"));
        } else if d.ends_with(".json") {
            let data =
                regorus::Value::from_json_file(&d).expect(&format!("failed to add data from {d}"));
            engine.add_data(data).expect(&format!("failed to add data from {d}"));
        } else {
            eprintln!("unknown data file {d}");
            return;
        }
    }

    let input = match cli.input {
        Some(i) => regorus::Value::from_json_file(i).expect("failed to read input"),
        _ => regorus::Value::new_object(),
    };

    let start = std::time::Instant::now();
    if let Some(rule) = cli.rule {
	let mut r = regorus::Value::Undefined;
	for _i in 0..cli.num_iterations {
	    engine.set_input(input.clone());
	    r = engine.eval_rule(rule.clone()).expect("failed to evaluate rule");
	}
	if cli.show_output {
	    println!("r = {}", r.to_json_str().expect("failed to serialize result"));
	}
    } else if let Some(query) = cli.query {
	let mut r = engine.eval_query(query.clone(), false).expect("failed to evaluate rule");
	for _i in 0..cli.num_iterations {
	    engine.set_input(input.clone());
	    r = engine.eval_query(query.clone(), false).expect("failed to evaluate query");
	}
	if cli.show_output {
	    println!("r = {}", serde_json::to_string_pretty(&r).expect("failed to serialize results"));
	}
    } else {
	eprintln!("either rule or query must be specified");
	return;
    }
    let average = start.elapsed().as_micros() as f64 / cli.num_iterations as f64;
    println!("average eval time = {average} microseconds");
}
