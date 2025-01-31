-------------------
-- Add missing token account association
-------------------

-- create a temporary table to hold the last transaction consensus timestamp when the migration is run
create table if not exists last_transaction (
  consensus_timestamp bigint
);

insert into last_transaction (consensus_timestamp)
select max(consensus_end) from record_file;

create or replace function add_missing_token_account_association() returns trigger as
$$
declare
    last_consensus_timestamp bigint;
    last_account_balance_file_timestamp bigint;
begin
    select consensus_timestamp into last_consensus_timestamp from last_transaction limit 1;
    select max(consensus_timestamp) into last_account_balance_file_timestamp from account_balance_file;
    if (last_consensus_timestamp is not null and last_account_balance_file_timestamp < last_consensus_timestamp) then
        -- make sure the new account balance file is not before the last transaction when the migration is run
        return null;
    end if;

    -- Importer has missed the following auto token-account associations:
    --   1. A token's treasury at the time of token creation
    --   2. A token's custom fee collector of either a fixed custom fee charged in the newly created token or a fractional
    --      fee, at the time of token creation
    -- Once the importer is patched, we can add associations for the token-account pairs in the account balance file but
    -- not in the token_account table
    insert into
        token_account (account_id, associated, created_timestamp, freeze_status, kyc_status, modified_timestamp, token_id)
    select
        tb.account_id,
        true,
        t.created_timestamp,
        case when t.freeze_key is null then 0
             else 2
            end,
        case when t.kyc_key is null then 0
             else 1
            end,
        t.created_timestamp,
        tb.token_id
    from token_balance tb
             join token t on t.token_id = tb.token_id
    where tb.consensus_timestamp = last_account_balance_file_timestamp
    on conflict do nothing;

    -- cleanup
    drop table if exists last_transaction;
    -- with cascade, also drop the trigger
    drop function add_missing_token_account_association() cascade;
    return null;
end;
$$ language plpgsql;

create trigger missing_token_account_trigger after insert
    on account_balance_file
    execute procedure add_missing_token_account_association();
