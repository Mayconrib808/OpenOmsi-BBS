fn main() {
    let a: Vec<String> = std::env::args().collect();
    let t = omsi_map::Terrain::load(std::path::Path::new(&a[1])).unwrap();
    println!("cells {}", t.cells);
    for arg in a[2..].chunks(2) {
        let (x, y): (f32, f32) = (arg[0].parse().unwrap(), arg[1].parse().unwrap());
        println!("local ({x}, {y}) -> height {:.3}", t.sample(x, y));
    }
}
