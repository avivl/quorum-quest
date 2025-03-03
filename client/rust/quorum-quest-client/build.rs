// client/rust/quorum-quest-client/build.rs

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Path to the proto file - adjust based on your repository structure
    let proto_file = "../../../api/proto/v1/quorum_quest_api.proto";
    
    println!("cargo:rerun-if-changed={}", proto_file);
    
    tonic_build::compile_protos(proto_file)?;
    
    Ok(())
}